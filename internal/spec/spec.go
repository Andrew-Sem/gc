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

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"

	"github.com/Andrew-Sem/gc/internal/app"
	"github.com/Andrew-Sem/gc/internal/domain"
	"github.com/Andrew-Sem/gc/internal/github"
	"github.com/Andrew-Sem/gc/internal/web"
)

const canaryToken = "spec-canary-pat-must-never-leak"

const fakeGitHubURL = "https://api.github.test"

type Suite struct {
	t        *testing.T
	contract contract
	covered  map[string]bool
}

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

func appHandler(deps app.Deps) http.Handler {
	return web.New(app.New(deps))
}

type Scenario struct {
	given  []domain.Fact
	github *fakeGitHub
	steps  []*step
	setup  []string
}

func newScenario() *Scenario {
	return &Scenario{github: &fakeGitHub{answers: map[string]githubAnswer{}}}
}

func (sc *Scenario) Given(facts ...domain.Fact) {
	sc.given = append(sc.given, facts...)
}

func (sc *Scenario) GitHubCanAccess(repos ...string) {
	sc.answerGitHub(githubAccessible, repos)
}

func (sc *Scenario) GitHubCannotAccess(repos ...string) {
	sc.answerGitHub(githubNotFound, repos)
}

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

func (sc *Scenario) When(r Request) *step {
	st := &step{request: r}
	sc.steps = append(sc.steps, st)
	return st
}

type Request struct {
	method string
	path   string
	fields Fields
}

type Fields map[string]string

func Get(path string) Request {
	return Request{method: http.MethodGet, path: path}
}

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

type step struct {
	request Request
	then    string
	covers  []string
	checks  []check
	setup   []string
}

type checkKind int

const (
	checkFacts checkKind = iota
	checkStatus
	checkHTML
)

type check struct {
	kind checkKind
	run  func(o *outcome) string
}

type outcome struct {
	status int
	body   string
	facts  []domain.Fact

	parsed bool
	doc    *html.Node
	docErr error
}

func (o *outcome) document() (*html.Node, error) {
	if !o.parsed {
		o.doc, o.docErr = html.Parse(strings.NewReader(o.body))
		o.parsed = true
	}
	return o.doc, o.docErr
}

func (st *step) Then(description string) *step {
	st.then = description
	return st
}

func (st *step) Covers(clauses ...string) *step {
	st.covers = append(st.covers, clauses...)
	return st
}

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

func (st *step) ExpectNoFacts() *step {
	st.checks = append(st.checks, check{kind: checkFacts, run: func(o *outcome) string {
		if len(o.facts) == 0 {
			return ""
		}
		return "ждали, что фактов не будет, а появились:\n" + list(o.facts)
	}})
	return st
}

func (st *step) ExpectHTML(selector string) *step {
	return st.expectHTML(selector, func(n int) string {
		if n > 0 {
			return ""
		}
		return "в ответе нет элемента " + selector
	})
}

func (st *step) ExpectNoHTML(selector string) *step {
	return st.expectHTML(selector, func(n int) string {
		if n == 0 {
			return ""
		}
		return fmt.Sprintf("в ответе не должно быть %s, а нашлось %d", selector, n)
	})
}

func (st *step) ExpectHTMLCount(selector string, want int) *step {
	return st.expectHTML(selector, func(n int) string {
		if n == want {
			return ""
		}
		return fmt.Sprintf("ждали %d элемент(ов) %s, нашлось %d", want, selector, n)
	})
}

func (st *step) expectHTML(source string, verdict func(found int) string) *step {
	sel, err := cascadia.ParseGroup(source)
	if err != nil {
		st.setup = append(st.setup, fmt.Sprintf("селектор %q: %v", source, err))
	}
	st.checks = append(st.checks, check{kind: checkHTML, run: func(o *outcome) string {
		if sel == nil {
			return ""
		}
		doc, err := o.document()
		if err != nil {
			return "ответ не разбирается как HTML: " + err.Error()
		}
		return verdict(len(cascadia.QueryAll(doc, sel)))
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
