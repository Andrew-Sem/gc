package spec

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/Andrew-Sem/gc/internal/app"
	"github.com/Andrew-Sem/gc/internal/domain"
	"github.com/Andrew-Sem/gc/internal/github"
)

func TestParseContract(t *testing.T) {
	c, err := parseContract("x.md", []byte(strings.Join([]string{
		"# Фича",
		"",
		"A-1. Первый пункт.",
		"",
		"A-1.1. Подпункт",
		"на двух строках.",
		"",
		"```",
		"A-9. Пример в блоке кода, не пункт.",
		"```",
	}, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, cl := range c.clauses {
		ids = append(ids, cl.id)
	}
	if want := []string{"A-1", "A-1.1"}; !slices.Equal(ids, want) {
		t.Fatalf("пункты %v, ждали %v", ids, want)
	}
	if got := c.clauses[1].text; got != "Подпункт на двух строках." {
		t.Fatalf("текст подпункта %q", got)
	}
}

func TestParseContractRejects(t *testing.T) {
	for name, text := range map[string]string{
		"повтор номера":       "A-1. Раз.\n\nA-1. Два.\n",
		"нет пунктов":         "# Пусто\n",
		"пункт без текста":    "A-1.\n",
		"пункт только в коде": "```\nA-1. Пример.\n```\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseContract("x.md", []byte(text)); err == nil {
				t.Fatal("ждали ошибку разбора")
			}
		})
	}
}

func TestUncovered(t *testing.T) {
	c := contract{clauses: []clause{{id: "A-1"}, {id: "A-1.1"}, {id: "A-2"}}}
	got := c.uncovered(map[string]bool{"A-1.1": true})
	if want := []string{"A-1", "A-2"}; !slices.Equal(got, want) {
		t.Fatalf("непокрытые %v, ждали %v", got, want)
	}
}

func TestValidate(t *testing.T) {
	c := contract{file: "x.md", clauses: []clause{{id: "A-1", text: "Пункт."}}}
	described := func(sc *Scenario) *step {
		return sc.When(Get("/")).Then("x").Covers("A-1").ExpectStatus(http.StatusOK)
	}
	cases := []struct {
		name  string
		build func(sc *Scenario)
		want  string
	}{
		{"нет When", func(sc *Scenario) {}, "ровно один When"},
		{"два When", func(sc *Scenario) {
			described(sc).ExpectNoFacts()
			described(sc).ExpectNoFacts()
		}, "ровно один When"},
		{"нет Then", func(sc *Scenario) {
			sc.When(Get("/")).Covers("A-1").ExpectStatus(http.StatusOK).ExpectNoFacts()
		}, "нет Then"},
		{"нет Covers", func(sc *Scenario) {
			sc.When(Get("/")).Then("x").ExpectStatus(http.StatusOK).ExpectNoFacts()
		}, "нет Covers"},
		{"пункта нет в договоре", func(sc *Scenario) {
			sc.When(Get("/")).Then("x").Covers("A-9").ExpectStatus(http.StatusOK).ExpectNoFacts()
		}, `Covers("A-9")`},
		{"нет проверки фактов", func(sc *Scenario) { described(sc) }, "ровно одна проверка фактов"},
		{"две проверки фактов", func(sc *Scenario) {
			described(sc).ExpectNoFacts().ExpectNoFacts()
		}, "ровно одна проверка фактов"},
		{"ExpectFacts без фактов", func(sc *Scenario) { described(sc).ExpectFacts() }, "ExpectNoFacts"},
		{"нет проверки статуса", func(sc *Scenario) {
			sc.When(Get("/")).Then("x").Covers("A-1").ExpectNoFacts()
		}, "ExpectStatus"},
		{"ответ GitHub задан дважды", func(sc *Scenario) {
			sc.GitHubCanAccess("acme/repo")
			sc.GitHubCannotAccess("acme/repo")
			described(sc).ExpectNoFacts()
		}, "дважды"},
		{"сломанный селектор", func(sc *Scenario) {
			described(sc).ExpectNoFacts().ExpectHTML(`li[data-repo="acme`)
		}, "селектор"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sc := newScenario()
			tc.build(sc)
			problems := strings.Join(validate(sc, c), "\n")
			if !strings.Contains(problems, tc.want) {
				t.Fatalf("ждали %q среди ошибок, получили:\n%s", tc.want, problems)
			}
		})
	}
	t.Run("правильный сценарий", func(t *testing.T) {
		sc := newScenario()
		described(sc).ExpectNoFacts().ExpectHTML(`li[data-repo="acme/repo"]`)
		if problems := validate(sc, c); len(problems) > 0 {
			t.Fatalf("лишние ошибки: %v", problems)
		}
	})
}

func TestExecute(t *testing.T) {
	connected := domain.RepositoryConnected{Repo: "acme/repo"}
	scenario := func(expect func(st *step)) *Scenario {
		sc := newScenario()
		expect(sc.When(PostForm("/repositories", Fields{"repo": "acme/repo"})).Then("x").Covers("A-1"))
		return sc
	}
	cases := []struct {
		name    string
		sc      *Scenario
		handler func(app.Deps) http.Handler
		want    string
	}{
		{
			"несуществующий адрес не проходит негативный сценарий",
			scenario(func(st *step) { st.ExpectStatus(http.StatusOK).ExpectNoFacts() }),
			func(app.Deps) http.Handler { return http.NotFoundHandler() },
			"ждали статус 200",
		},
		{
			"факт записан, но сервер упал",
			scenario(func(st *step) { st.ExpectStatus(http.StatusOK).ExpectFacts(connected) }),
			reply(http.StatusInternalServerError, "", connected),
			"ждали статус 200",
		},
		{
			"записан не тот факт",
			scenario(func(st *step) { st.ExpectStatus(http.StatusOK).ExpectFacts(connected) }),
			reply(http.StatusOK, "", domain.RepositoryConnected{Repo: "other/repo"}),
			"ждали факты",
		},
		{
			"вопрос GitHub про неописанный репозиторий",
			scenario(func(st *step) { st.ExpectStatus(http.StatusOK).ExpectNoFacts() }),
			askGitHub("other/repo"),
			`"other/repo"`,
		},
		{
			"нет нужного элемента в HTML",
			scenario(func(st *step) {
				st.ExpectStatus(http.StatusOK).ExpectNoFacts().ExpectHTML(`li[data-repo="acme/repo"]`)
			}),
			reply(http.StatusOK, `<ul><li data-repo="other/repo">other/repo</li></ul>`),
			"нет элемента",
		},
		{
			"лишний элемент в HTML",
			scenario(func(st *step) {
				st.ExpectStatus(http.StatusOK).ExpectNoFacts().ExpectNoHTML(`[data-spec="error"]`)
			}),
			reply(http.StatusOK, `<p data-spec="error">упало</p>`),
			"не должно быть",
		},
		{
			"PAT в ответе",
			scenario(func(st *step) { st.ExpectStatus(http.StatusOK).ExpectNoFacts() }),
			reply(http.StatusOK, "token="+canaryToken),
			"PAT попал в ответ",
		},
		{
			"PAT в логах",
			scenario(func(st *step) { st.ExpectStatus(http.StatusOK).ExpectNoFacts() }),
			logToken(),
			"PAT попал в логи",
		},
		{
			"всё сошлось",
			scenario(func(st *step) {
				st.ExpectStatus(http.StatusOK).
					ExpectFacts(connected).
					ExpectHTMLCount(`li[data-repo]`, 1).
					ExpectNoHTML(`[data-spec="error"]`)
			}),
			reply(http.StatusOK, `<ul><li data-repo="acme/repo">acme/repo</li></ul>`, connected),
			"",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := execute(t.Context(), tc.sc, tc.handler)
			switch {
			case tc.want == "" && report != "":
				t.Fatalf("ждали, что сценарий пройдёт, а отчёт:\n%s", report)
			case tc.want != "" && !strings.Contains(report, tc.want):
				t.Fatalf("ждали %q в отчёте, а отчёт:\n%s", tc.want, report)
			}
		})
	}
}

func TestFakeGitHubThroughRealClient(t *testing.T) {
	sc := newScenario()
	sc.GitHubCanAccess("acme/open")
	sc.GitHubCannotAccess("acme/closed")
	sc.GitHubFails("acme/down")
	client := github.New(fakeGitHubURL, canaryToken, &http.Client{Transport: handlerTransport{sc.github}})
	ctx := t.Context()

	if ok, err := client.RepositoryAccessible(ctx, "acme/open"); !ok || err != nil {
		t.Fatalf("acme/open: %v, %v", ok, err)
	}
	if ok, err := client.RepositoryAccessible(ctx, "acme/closed"); ok || err != nil {
		t.Fatalf("acme/closed: %v, %v", ok, err)
	}
	_, err := client.RepositoryAccessible(ctx, "acme/down")
	if err == nil || strings.Contains(err.Error(), canaryToken) {
		t.Fatalf("acme/down: ждали ошибку без токена, получили %v", err)
	}
	if problems := sc.github.reported(); len(problems) != 0 {
		t.Fatalf("описанные репозитории попали в отчёт: %v", problems)
	}
}

func TestRequestString(t *testing.T) {
	if got := Get("/").String(); got != "GET /" {
		t.Fatalf("%q", got)
	}
	got := PostForm("/repositories", Fields{"repo": "acme/repo", "b": "2"}).String()
	if want := "POST /repositories b=2&repo=acme/repo"; got != want {
		t.Fatalf("%q, ждали %q", got, want)
	}
}

func reply(status int, body string, facts ...domain.Fact) func(app.Deps) http.Handler {
	return func(deps app.Deps) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := deps.Journal.Append(r.Context(), facts...); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		})
	}
}

func askGitHub(repo string) func(app.Deps) http.Handler {
	return func(deps app.Deps) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = deps.GitHub.RepositoryAccessible(r.Context(), repo)
			w.WriteHeader(http.StatusOK)
		})
	}
}

func logToken() func(app.Deps) http.Handler {
	return func(deps app.Deps) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			deps.Log.Info("отладка", "token", canaryToken)
			w.WriteHeader(http.StatusOK)
		})
	}
}
