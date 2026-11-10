package extensions

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/internal/store"
	"github.com/Andrew-Sem/gc/specs/contract"
	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

type earlierStart struct {
	requests []spec.Request
	killed   bool
}

type journalRow struct {
	kind, data string
}

type journalFile struct {
	dir  string
	path string

	seed            []contract.Fact
	rows            []journalRow
	contents        *string
	earlier         []earlierStart
	otherRunning    bool
	failingWrite    *spec.Request
	expectNoSuccess bool
	breakJournal    func() error

	elements      []string
	noElements    []string
	expectJournal bool
	journalKinds  []string
	journal       []contract.Fact
	rowCount      *int
	fileWithout   []string
	expectMode    fs.FileMode
	unchanged     bool
}

func newJournalFile() journalFile {
	dir, err := os.MkdirTemp("", "gc-journal-")
	if err != nil {
		panic(err)
	}
	return journalFile{dir: dir, path: filepath.Join(dir, "gc.db")}
}

func (l *Launch) GivenJournal(facts ...contract.Fact) *Launch {
	l.seed = append(l.seed, facts...)
	return l
}

func (l *Launch) GivenJournalRow(kind, data string) *Launch {
	l.rows = append(l.rows, journalRow{kind: kind, data: data})
	return l
}

func (l *Launch) GivenJournalFileContents(contents string) *Launch {
	l.contents = &contents
	return l
}

func (l *Launch) GivenEarlierStart(requests ...spec.Request) *Launch {
	l.earlier = append(l.earlier, earlierStart{requests: requests})
	return l
}

func (l *Launch) GivenEarlierStartKilled(requests ...spec.Request) *Launch {
	l.earlier = append(l.earlier, earlierStart{requests: requests, killed: true})
	return l
}

func (l *Launch) GivenAnotherGCRunning() *Launch {
	l.otherRunning = true
	return l
}

func (l *Launch) WhenWriteFails(r spec.Request) *Launch {
	l.failingWrite = &r
	l.whenCalls++
	return l
}

func (l *Launch) WhenJournalRemovedBefore(r spec.Request) *Launch {
	l.breakJournal = func() error { return os.Remove(l.path) }
	return l.WhenWriteFails(r)
}

func (l *Launch) WhenJournalReplacedBefore(r spec.Request) *Launch {
	l.breakJournal = func() error {
		if err := os.Rename(l.path, l.path+".old"); err != nil {
			return err
		}
		return os.WriteFile(l.path, nil, 0o600)
	}
	return l.WhenWriteFails(r)
}

func (l *Launch) ExpectElement(selector string) *Launch {
	l.elements = append(l.elements, selector)
	return l
}

func (l *Launch) ExpectNoElement(selector string) *Launch {
	l.noElements = append(l.noElements, selector)
	return l
}

func (l *Launch) ExpectJournal(facts ...contract.Fact) *Launch {
	l.expectJournal = true
	l.journal = facts
	return l
}

func (l *Launch) ExpectJournalRows(count int) *Launch {
	l.rowCount = &count
	return l
}

func (l *Launch) ExpectJournalFileWithout(text string) *Launch {
	l.fileWithout = append(l.fileWithout, text)
	return l
}

func (l *Launch) ExpectJournalFileMode(mode fs.FileMode) *Launch {
	l.expectMode = mode
	return l
}

func (l *Launch) ExpectJournalFileUnchanged() *Launch {
	l.unchanged = true
	return l
}

func (l *Launch) JournalPath() string {
	return l.path
}

func (l *Launch) PathInMissingFolder() string {
	return filepath.Join(l.dir, "missing", "gc.db")
}

func (l *Launch) OpenAIKey() string {
	return fakeOpenAIKey
}

func EveryKindOfFact() []contract.Fact {
	at := At("1999-12-31 22:00")
	return []contract.Fact{
		contract.RepositoryConnected{FullName: "acme/api"},
		contract.RepositorySettingsChanged{Repository: "acme/api", Pace: 1, Limit: 12, Merge: true, At: at},
		contract.SettingsChanged{OpenAIBaseURL: "http://127.0.0.1:8317/v1", At: at},
		contract.RunRequested{Repository: "acme/api"},
		contract.SweepRequested{Repository: "acme/api"},
		contract.MergeApproved{Repository: "acme/api", Number: 7, HeadSHA: "abc123"},
		contract.MergeDeclined{Repository: "acme/api", Number: 7, Reason: "draft"},
		contract.MergeFailed{Repository: "acme/api", Number: 7, Reason: "GitHub answered 405"},
		contract.AdmissionApproved{Repository: "acme/api", OpenAIBaseURL: "http://127.0.0.1:8317/v1"},
		contract.AdmissionDeclined{Repository: "acme/api", Open: 12, Limit: 12, Running: 3},
		contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at},
		contract.PrCreated{Repository: "acme/api", Number: 7},
		contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomePR, Number: 7, Explanation: "None", Tokens: contract.TokenUsage{Input: 3, Cached: 2, Output: 1}, At: at.Add(time.Minute)},
		contract.PrStateChanged{
			Number:     7,
			Repository: "acme/api",
			HeadSHA:    "abc123",
			Base:       "main",
			Merged:     contract.MergedYes,
			Draft:      true,
			Open:       true,
			Mergeable:  contract.MergeableUnknown,
			CreatedAt:  at,
			UpdatedAt:  at.Add(time.Nanosecond),
		},
		contract.CheckRunStateChanged{Repository: "acme/api", Number: 7, CheckRun: contract.CheckRun{
			ID: 9, Name: "test", AppID: 15368, HeadSHA: "abc123", Status: "completed", Conclusion: "success",
			StartedAt: at, CompletedAt: at.Add(time.Minute),
		}},
		contract.RequiredChecksObserved{Repository: "acme/api", Number: 7, HeadSHA: "abc123", Required: []contract.RequiredCheck{{Name: "test", AppID: 15368}, {Name: "lint"}}},
		contract.RepositoryDisconnected{FullName: "acme/api"},
	}
}

func (l *Launch) prepareJournalFile() error {
	if l.contents != nil {
		return os.WriteFile(l.path, []byte(*l.contents), 0o600)
	}
	if len(l.seed)+len(l.rows) == 0 {
		return nil
	}
	journal, err := store.Open(l.path, func(err error) { panic(err) })
	if err != nil {
		return err
	}
	journal.Append(l.seed...)
	if err := journal.Close(); err != nil {
		return err
	}
	if len(l.rows) == 0 {
		return nil
	}
	db, err := sql.Open("sqlite", l.path)
	if err != nil {
		return err
	}
	defer db.Close()
	for _, row := range l.rows {
		if _, err := db.Exec("INSERT INTO facts (kind, data) VALUES (?, ?)", row.kind, row.data); err != nil {
			return err
		}
	}
	return nil
}

func (l *Launch) removeJournalFile() {
	os.Chmod(l.dir, 0o700)
	os.RemoveAll(l.dir)
}

const OtherGCRepository = "acme/other"

type process struct {
	cmd    *exec.Cmd
	addr   string
	output bytes.Buffer
	exited chan error
	done   bool
}

func (l *Launch) start(extraEnv ...string) (*process, error) {
	p := &process{cmd: exec.Command(l.binary), exited: make(chan error, 1)}
	p.cmd.Env = append(l.environ(), extraEnv...)
	p.cmd.Stdout = &p.output
	p.cmd.Stderr = &p.output
	if err := p.cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		p.exited <- p.cmd.Wait()
		close(p.exited)
	}()
	return p, nil
}

func (p *process) stop() {
	if p.done {
		return
	}
	p.done = true
	p.cmd.Process.Signal(os.Interrupt)
	select {
	case <-p.exited:
	case <-time.After(startupLimit):
		p.cmd.Process.Kill()
		<-p.exited
	}
}

func (p *process) kill() {
	p.done = true
	p.cmd.Process.Kill()
	<-p.exited
}

func (l *Launch) startEarlier(earlier earlierStart) spec.Problem {
	p, err := l.start()
	if err != nil {
		return launchProblem(fmt.Sprintf("cannot start the earlier gc: %v", err))
	}
	if ready, _ := waitServing(l.addr, p.exited); !ready {
		p.stop()
		return launchProblem(fmt.Sprintf("the earlier gc did not start: %s", p.output.String()))
	}
	for _, request := range earlier.requests {
		if _, err := l.do(l.addr, request); err != nil {
			p.stop()
			return launchProblem(fmt.Sprintf("cannot send the earlier request: %v", err))
		}
	}
	if earlier.killed {
		p.kill()
	} else {
		p.stop()
	}
	return nil
}

func (l *Launch) do(addr string, r spec.Request) (int, error) {
	req, err := r.HTTPRequest("http://" + addr)
	if err != nil {
		return 0, err
	}
	asOperator(req)
	resp, err := (&http.Client{Timeout: 2 * longerThanAnyTimeout}).Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}

func (l *Launch) startOther() (*process, spec.Problem) {
	addr := freeAddress()
	other, err := l.start("GC_ADDR=" + addr)
	if err != nil {
		return nil, launchProblem(fmt.Sprintf("cannot start the other gc: %v", err))
	}
	if ready, _ := waitServing(addr, other.exited); !ready {
		other.stop()
		return nil, launchProblem(fmt.Sprintf("the other gc did not start: %s", other.output.String()))
	}
	other.addr = addr
	return other, nil
}

func (l *Launch) otherStillWorks(other *process) []spec.Problem {
	defer other.stop()
	status, err := l.do(other.addr, spec.Post("/repositories").WithForm("repo", OtherGCRepository))
	if err != nil {
		return []spec.Problem{launchProblem(fmt.Sprintf("the other gc stopped working: %v", err))}
	}
	if status != http.StatusOK {
		return []spec.Problem{launchProblem(fmt.Sprintf("the other gc answered %d, want 200", status))}
	}
	return nil
}

func (l *Launch) breakWrites(exited <-chan error) []spec.Problem {
	if ready, _ := waitServing(l.addr, exited); !ready {
		return []spec.Problem{launchProblem("gc did not start")}
	}
	breakJournal := l.breakJournal
	if breakJournal == nil {
		breakJournal = func() error { return os.Chmod(l.dir, 0o500) }
	}
	if err := breakJournal(); err != nil {
		return []spec.Problem{launchProblem(fmt.Sprintf("cannot break the journal: %v", err))}
	}
	defer os.Chmod(l.dir, 0o700)
	status, err := l.do(l.addr, *l.failingWrite)
	if l.expectNoSuccess && err == nil && status < 300 {
		return []spec.Problem{launchProblem(fmt.Sprintf("expected no success for a write that failed, got %d", status))}
	}
	return nil
}

func (l *Launch) pageProblems(body io.Reader) []spec.Problem {
	page, err := html.Parse(body)
	if err != nil {
		return []spec.Problem{launchProblem(fmt.Sprintf("cannot read the page: %v", err))}
	}
	var problems []spec.Problem
	check := func(selector string, want bool) {
		sel, err := cascadia.Compile(selector)
		if err != nil {
			problems = append(problems, launchProblem(fmt.Sprintf("invalid selector %s: %v", selector, err)))
			return
		}
		if found := cascadia.Query(page, sel) != nil; found != want {
			problems = append(problems, launchProblem(fmt.Sprintf("element %s found: %v, want %v", selector, found, want)))
		}
	}
	for _, selector := range l.elements {
		check(selector, true)
	}
	for _, selector := range l.noElements {
		check(selector, false)
	}
	return problems
}

func (l *Launch) journalFileProblems() []spec.Problem {
	var problems []spec.Problem
	if l.unchanged {
		got, err := os.ReadFile(l.path)
		if err != nil || string(got) != *l.contents {
			problems = append(problems, launchProblem(fmt.Sprintf("expected the journal file unchanged, got %q (%v)", got, err)))
		}
	}
	if l.expectMode != 0 {
		info, err := os.Stat(l.path)
		if err != nil {
			problems = append(problems, launchProblem(fmt.Sprintf("cannot read the journal file: %v", err)))
		} else if info.Mode().Perm() != l.expectMode {
			problems = append(problems, launchProblem(fmt.Sprintf("expected the journal file mode %v, got %v", l.expectMode, info.Mode().Perm())))
		}
	}
	if len(l.fileWithout) > 0 {
		got, err := os.ReadFile(l.path)
		if err != nil {
			problems = append(problems, launchProblem(fmt.Sprintf("cannot read the journal file: %v", err)))
		}
		for _, text := range l.fileWithout {
			if strings.Contains(string(got), text) {
				problems = append(problems, launchProblem(fmt.Sprintf("expected the journal file not to contain %q", text)))
			}
		}
	}
	if l.rowCount != nil {
		problems = append(problems, l.rowCountProblems()...)
	}
	if l.expectJournal {
		problems = append(problems, l.journalProblems()...)
	}
	if l.journalKinds != nil {
		problems = append(problems, l.journalKindProblems()...)
	}
	return problems
}

func (l *Launch) rowCountProblems() []spec.Problem {
	db, err := sql.Open("sqlite", l.path)
	if err != nil {
		return []spec.Problem{launchProblem(fmt.Sprintf("cannot open the journal file: %v", err))}
	}
	defer db.Close()
	var got int
	if err := db.QueryRow("SELECT count(*) FROM facts").Scan(&got); err != nil {
		return []spec.Problem{launchProblem(fmt.Sprintf("cannot count the facts: %v", err))}
	}
	if got != *l.rowCount {
		return []spec.Problem{launchProblem(fmt.Sprintf("expected %d rows in the journal file, got %d", *l.rowCount, got))}
	}
	return nil
}

func (l *Launch) journalProblems() []spec.Problem {
	journal, err := store.Open(l.path, func(err error) { panic(err) })
	if err != nil {
		return []spec.Problem{launchProblem(fmt.Sprintf("cannot open the journal file: %v", err))}
	}
	defer journal.Close()
	if got := journal.Facts(); !reflect.DeepEqual(got, l.journal) {
		return []spec.Problem{launchProblem(fmt.Sprintf("expected the journal %#v, got %#v", l.journal, got))}
	}
	return nil
}
