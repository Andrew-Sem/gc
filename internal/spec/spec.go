// Package spec — маленький фреймворк исполняемых спек: Given / When / Then поверх журнала
// фактов и HTTP.
//
// Сценарии одной фичи привязаны к её договору — markdown-файлу с нумерованными пунктами.
// Сценарий собирается декларативно, а запускается целиком после сборки: журнал с фактами из
// Given → приложение с настоящим клиентом GitHub, который ходит в поддельный GitHub API →
// один запрос из When → проверки Expect… и сквозная проверка, что PAT нигде не всплыл.
package spec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"golang.org/x/net/html"

	"github.com/andrew-semyonov/garbage-collector/internal/app"
	"github.com/andrew-semyonov/garbage-collector/internal/domain"
	"github.com/andrew-semyonov/garbage-collector/internal/github"
	"github.com/andrew-semyonov/garbage-collector/internal/web"
)

// canaryToken — PAT, который получает клиент GitHub в каждом сценарии. Если он всплывёт в
// журнале, ответе или логах, сценарий падает (REPO-4). Формат намеренно не похож на
// настоящий токен GitHub, чтобы сканеры секретов не принимали его за утечку.
const canaryToken = "spec-canary-pat-must-never-leak"

// fakeGitHubURL — адрес поддельного GitHub API. Запросы до сети не доходят.
const fakeGitHubURL = "https://api.github.test"

// Suite — сценарии одной фичи и её договор.
type Suite struct {
	t        *testing.T
	contract contract
	covered  map[string]bool
}

// Feature читает договор фичи. Путь считается от файла теста, а не от текущего каталога.
// После всех сценариев в отчёт попадают пункты договора, которые ещё не покрыты.
func Feature(t *testing.T, contractFile string) *Suite {
	t.Helper()
	_, testFile, _, _ := runtime.Caller(1)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(testFile), contractFile))
	if err != nil {
		t.Fatalf("договор не прочитан: %v", err)
	}
	c, err := parseContract(contractFile, data)
	if err != nil {
		t.Fatalf("договор %s: %v", contractFile, err)
	}
	s := &Suite{t: t, contract: c, covered: map[string]bool{}}
	t.Cleanup(func() {
		if missing := c.uncovered(s.covered); len(missing) > 0 {
			t.Logf("%s: пункты без спек: %s", contractFile, strings.Join(missing, ", "))
		}
	})
	return s
}

// Scenario описывает сценарий и сразу его запускает.
func (s *Suite) Scenario(name string, build func(sc *Scenario)) {
	_, file, line, _ := runtime.Caller(1)
	where := fmt.Sprintf("%s:%d", filepath.Base(file), line)
	s.t.Run(name, func(t *testing.T) {
		sc := newScenario()
		build(sc)
		if problems := validate(sc, s.contract); len(problems) > 0 {
			t.Fatalf("%s: сценарий описан неправильно:\n- %s", where, strings.Join(problems, "\n- "))
		}
		for _, clause := range sc.steps[0].covers {
			s.covered[clause] = true
		}
		if report := execute(t.Context(), sc, appHandler); report != "" {
			t.Errorf("%s: %s", where, report)
		}
	})
}

// appHandler собирает приложение так же, как cmd/gc.
func appHandler(deps app.Deps) http.Handler {
	return web.New(app.New(deps))
}

// Scenario собирает один сценарий до запуска.
type Scenario struct {
	given  []domain.Fact
	github *fakeGitHub
	steps  []*step
	setup  []string // ошибки в описании Given; всплывают при проверке сценария
}

func newScenario() *Scenario {
	return &Scenario{github: &fakeGitHub{answers: map[string]githubAnswer{}}}
}

// Given кладёт факты в журнал до запроса.
func (sc *Scenario) Given(facts ...domain.Fact) {
	sc.given = append(sc.given, facts...)
}

// GitHubCanAccess: репозиторий существует, и токен его видит.
func (sc *Scenario) GitHubCanAccess(repos ...string) {
	sc.answerGitHub(githubAccessible, repos)
}

// GitHubCannotAccess: репозитория нет или токен его не видит.
func (sc *Scenario) GitHubCannotAccess(repos ...string) {
	sc.answerGitHub(githubNotFound, repos)
}

// GitHubFails: GitHub отвечает ошибкой и, как худший случай, возвращает в теле ошибки
// заголовок с токеном.
func (sc *Scenario) GitHubFails(repos ...string) {
	sc.answerGitHub(githubDown, repos)
}

func (sc *Scenario) answerGitHub(answer githubAnswer, repos []string) {
	for _, repo := range repos {
		if _, ok := sc.github.answers[repo]; ok {
			sc.setup = append(sc.setup, fmt.Sprintf("ответ GitHub для %q задан дважды", repo))
			continue
		}
		sc.github.answers[repo] = answer
	}
}

// When задаёт запрос сценария. Запрос в сценарии ровно один.
func (sc *Scenario) When(r Request) *step {
	st := &step{request: r}
	sc.steps = append(sc.steps, st)
	return st
}

// Request — HTTP-запрос, который отправил бы браузер.
type Request struct {
	method string
	path   string
	fields Fields
}

// Fields — поля формы.
type Fields map[string]string

// Get открывает страницу.
func Get(path string) Request {
	return Request{method: http.MethodGet, path: path}
}

// PostForm отправляет форму методом POST, как кнопка на странице.
func PostForm(path string, fields Fields) Request {
	return Request{method: http.MethodPost, path: path, fields: fields}
}

func (r Request) String() string {
	if len(r.fields) == 0 {
		return r.method + " " + r.path
	}
	var pairs []string
	for _, key := range slices.Sorted(maps.Keys(r.fields)) {
		pairs = append(pairs, key+"="+r.fields[key])
	}
	return r.method + " " + r.path + " " + strings.Join(pairs, "&")
}

func (r Request) build(ctx context.Context) *http.Request {
	var body io.Reader
	if r.fields != nil {
		values := url.Values{}
		for key, value := range r.fields {
			values.Set(key, value)
		}
		body = strings.NewReader(values.Encode())
	}
	req := httptest.NewRequestWithContext(ctx, r.method, r.path, body)
	if r.fields != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return req
}

// step — ожидания после запроса.
type step struct {
	request Request
	then    string
	covers  []string
	checks  []check
	setup   []string // ошибки в описании ожиданий
}

type checkKind int

const (
	checkFacts checkKind = iota
	checkStatus
	checkHTML
)

// check возвращает описание расхождения или пустую строку, если всё сошлось.
type check struct {
	kind checkKind
	run  func(o *outcome) string
}

// outcome — что получилось после запроса.
type outcome struct {
	status int
	body   string
	facts  []domain.Fact // появились в журнале после запроса

	parsed bool
	doc    *html.Node
	docErr error
}

// document разбирает ответ как HTML один раз на все проверки.
func (o *outcome) document() (*html.Node, error) {
	if !o.parsed {
		o.doc, o.docErr = html.Parse(strings.NewReader(o.body))
		o.parsed = true
	}
	return o.doc, o.docErr
}

// Then — подпись для людей. Сама ничего не проверяет: проверяют вызовы Expect….
func (st *step) Then(description string) *step {
	st.then = description
	return st
}

// Covers — пункты договора, которые проверяет сценарий.
func (st *step) Covers(clauses ...string) *step {
	st.covers = append(st.covers, clauses...)
	return st
}

// ExpectStatus — HTTP-статус ответа.
func (st *step) ExpectStatus(code int) *step {
	st.checks = append(st.checks, check{kind: checkStatus, run: func(o *outcome) string {
		if o.status == code {
			return ""
		}
		return fmt.Sprintf("ждали статус %d %s, пришёл %d %s",
			code, http.StatusText(code), o.status, http.StatusText(o.status))
	}})
	return st
}

// ExpectFacts — после запроса в журнале появились ровно эти факты, в этом порядке.
func (st *step) ExpectFacts(facts ...domain.Fact) *step {
	if len(facts) == 0 {
		st.setup = append(st.setup, "ExpectFacts без фактов — для «ничего не записано» есть ExpectNoFacts")
	}
	st.checks = append(st.checks, check{kind: checkFacts, run: func(o *outcome) string {
		if sameFacts(o.facts, facts) {
			return ""
		}
		return "ждали факты:\n" + list(facts) + "\nпоявились:\n" + list(o.facts)
	}})
	return st
}

// ExpectNoFacts — запрос ничего не записал в журнал.
func (st *step) ExpectNoFacts() *step {
	st.checks = append(st.checks, check{kind: checkFacts, run: func(o *outcome) string {
		if len(o.facts) == 0 {
			return ""
		}
		return "ждали, что фактов не будет, а появились:\n" + list(o.facts)
	}})
	return st
}

// ExpectHTML — в ответе есть хотя бы один элемент под селектор.
func (st *step) ExpectHTML(selector string) *step {
	return st.expectHTML(selector, func(n int) string {
		if n > 0 {
			return ""
		}
		return "в ответе нет элемента " + selector
	})
}

// ExpectNoHTML — в ответе нет ни одного элемента под селектор.
func (st *step) ExpectNoHTML(selector string) *step {
	return st.expectHTML(selector, func(n int) string {
		if n == 0 {
			return ""
		}
		return fmt.Sprintf("в ответе не должно быть %s, а нашлось %d", selector, n)
	})
}

// ExpectHTMLCount — в ответе ровно столько элементов под селектор.
func (st *step) ExpectHTMLCount(selector string, want int) *step {
	return st.expectHTML(selector, func(n int) string {
		if n == want {
			return ""
		}
		return fmt.Sprintf("ждали %d элемент(ов) %s, нашлось %d", want, selector, n)
	})
}

func (st *step) expectHTML(source string, verdict func(found int) string) *step {
	sel, err := compileSelector(source)
	if err != nil {
		st.setup = append(st.setup, fmt.Sprintf("селектор %q: %v", source, err))
	}
	st.checks = append(st.checks, check{kind: checkHTML, run: func(o *outcome) string {
		if sel == nil {
			return "" // ошибка селектора уже в описании сценария
		}
		doc, err := o.document()
		if err != nil {
			return "ответ не разбирается как HTML: " + err.Error()
		}
		return verdict(len(sel.matchAll(doc)))
	}})
	return st
}

func (st *step) count(kind checkKind) int {
	n := 0
	for _, c := range st.checks {
		if c.kind == kind {
			n++
		}
	}
	return n
}

// validate проверяет описание сценария до запуска и возвращает все найденные ошибки.
func validate(sc *Scenario, c contract) []string {
	problems := slices.Clone(sc.setup)
	if len(sc.steps) != 1 {
		return append(problems, fmt.Sprintf("нужен ровно один When, а их %d", len(sc.steps)))
	}
	st := sc.steps[0]
	problems = append(problems, st.setup...)
	if st.then == "" {
		problems = append(problems, "нет Then — подпись нужна, чтобы по отчёту было понятно, что проверяем")
	}
	if len(st.covers) == 0 {
		problems = append(problems, "нет Covers — каждый сценарий проверяет пункт договора")
	}
	for _, clause := range st.covers {
		if !c.has(clause) {
			problems = append(problems, fmt.Sprintf("Covers(%q) — такого пункта нет в договоре %s", clause, c.file))
		}
	}
	if n := st.count(checkFacts); n != 1 {
		problems = append(problems, fmt.Sprintf("нужна ровно одна проверка фактов — ExpectFacts или ExpectNoFacts, а их %d", n))
	}
	if n := st.count(checkStatus); n != 1 {
		problems = append(problems, fmt.Sprintf("нужна ровно одна проверка ExpectStatus, а их %d", n))
	}
	return problems
}

// execute прогоняет сценарий на свежем мире и возвращает отчёт о расхождениях.
// Пустая строка — всё сошлось.
func execute(ctx context.Context, sc *Scenario, newHandler func(app.Deps) http.Handler) string {
	st := sc.steps[0]
	j := &journal{}
	if err := j.Append(ctx, sc.given...); err != nil {
		return "Given не записался в журнал: " + err.Error()
	}
	before := j.snapshot()

	var logs bytes.Buffer
	gh := &http.Client{Transport: handlerTransport{sc.github}}
	handler := newHandler(app.Deps{
		Journal: j,
		GitHub:  github.New(fakeGitHubURL, canaryToken, gh),
		Log:     slog.New(slog.NewTextHandler(&logs, nil)),
	})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, st.request.build(ctx))

	after := j.snapshot()
	if len(after) < len(before) || !sameFacts(after[:len(before)], before) {
		return "история в журнале изменилась — в него можно только дописывать"
	}
	o := &outcome{status: response.Code, body: response.Body.String(), facts: after[len(before):]}

	var problems []string
	for _, c := range st.checks {
		if problem := c.run(o); problem != "" {
			problems = append(problems, problem)
		}
	}
	problems = append(problems, sc.github.reported()...)
	problems = append(problems, leaks(after, response, logs.String())...)
	if len(problems) == 0 {
		return ""
	}
	return fmt.Sprintf("%s — %s\nWhen %s → %d %s%s\n\n%s",
		strings.Join(st.covers, ", "), st.then,
		st.request, o.status, http.StatusText(o.status), excerpt(o.body),
		strings.Join(problems, "\n\n"))
}

// leaks ищет PAT везде, куда он не должен попадать (REPO-4). Проверка идёт в каждом сценарии.
func leaks(facts []domain.Fact, response *httptest.ResponseRecorder, logs string) []string {
	var found []string
	if strings.Contains(list(facts), canaryToken) {
		found = append(found, "PAT попал в журнал")
	}
	if strings.Contains(response.Body.String(), canaryToken) {
		found = append(found, "PAT попал в ответ")
	}
	for name, values := range response.Header() {
		if strings.Contains(strings.Join(values, " "), canaryToken) {
			found = append(found, "PAT попал в заголовок ответа "+name)
		}
	}
	if strings.Contains(logs, canaryToken) {
		found = append(found, "PAT попал в логи")
	}
	return found
}

// journal — журнал фактов в памяти. Он часть доверенной обвязки: спеки читают факты через
// него, поэтому реализация приложения его не трогает. Когда появится SQLite, спеки перейдут на
// настоящее хранилище.
type journal struct {
	mu    sync.Mutex
	facts []domain.Fact
}

func (j *journal) Append(_ context.Context, facts ...domain.Fact) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.facts = append(j.facts, facts...)
	return nil
}

func (j *journal) Facts(_ context.Context) ([]domain.Fact, error) {
	return j.snapshot(), nil
}

func (j *journal) snapshot() []domain.Fact {
	j.mu.Lock()
	defer j.mu.Unlock()
	return slices.Clone(j.facts)
}

type githubAnswer int

const (
	githubAccessible githubAnswer = iota + 1
	githubNotFound
	githubDown
)

// fakeGitHub — GitHub API из сценария. Требует токен и отвечает только про описанные
// репозитории; остальное попадает в отчёт, а не превращается молча в «недоступен».
type fakeGitHub struct {
	mu       sync.Mutex
	answers  map[string]githubAnswer
	problems []string
}

func (f *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer "+canaryToken {
		f.problems = append(f.problems, fmt.Sprintf("запрос в GitHub %s %s пришёл без токена", r.Method, r.URL.Path))
		http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
		return
	}
	repo, ok := strings.CutPrefix(r.URL.Path, "/repos/")
	if r.Method != http.MethodGet || !ok {
		f.problems = append(f.problems, fmt.Sprintf("неожиданный запрос в GitHub: %s %s", r.Method, r.URL.Path))
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		return
	}
	switch f.answers[repo] {
	case githubAccessible:
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"full_name":%q}`, repo)
	case githubNotFound:
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	case githubDown:
		http.Error(w, "upstream failed, request had Authorization: "+r.Header.Get("Authorization"), http.StatusBadGateway)
	default:
		f.problems = append(f.problems, fmt.Sprintf(
			"приложение спросило GitHub про %q, а сценарий про этот репозиторий ничего не сказал", repo))
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	}
}

func (f *fakeGitHub) reported() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.problems)
}

// handlerTransport отдаёт запросы клиента прямо в обработчик, без сети.
type handlerTransport struct {
	handler http.Handler
}

func (t handlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	t.handler.ServeHTTP(recorder, req)
	response := recorder.Result()
	response.Request = req
	return response, nil
}

func sameFacts(a, b []domain.Fact) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !reflect.DeepEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

func list(facts []domain.Fact) string {
	if len(facts) == 0 {
		return "  (ничего)"
	}
	lines := make([]string, len(facts))
	for i, fact := range facts {
		lines[i] = fmt.Sprintf("  %#v", fact)
	}
	return strings.Join(lines, "\n")
}

// excerpt — начало тела ответа для отчёта: по нему видно, что ответило приложение.
func excerpt(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	if runes := []rune(body); len(runes) > 300 {
		body = string(runes[:300]) + "…"
	}
	return "\n" + body
}
