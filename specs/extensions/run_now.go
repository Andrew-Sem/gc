package extensions

import (
	"fmt"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/internal/store"
)

const (
	runEndLimit  = 20 * time.Second
	decisionWait = time.Second
)

func (l *Launch) ThenAWhilePasses() *Launch {
	l.pause = decisionWait
	return l
}

func (l *Launch) ThenTheRunEnds() *Launch {
	l.waitRun = true
	return l
}

func (l *Launch) ExpectJournalKinds(kinds ...string) *Launch {
	l.journalKinds = kinds
	return l
}

func (l *Launch) waitRunEnd() []spec.Problem {
	deadline := time.Now().Add(runEndLimit)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + l.addr + "/runs/panel")
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			page := string(body)
			if strings.Contains(page, "data-run=") && !strings.Contains(page, ">running<") {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return []spec.Problem{launchProblem(fmt.Sprintf("expected the run to end within %s", runEndLimit))}
}

func (l *Launch) journalKindProblems() []spec.Problem {
	journal, err := store.Open(l.path, func(err error) { panic(err) })
	if err != nil {
		return []spec.Problem{launchProblem(fmt.Sprintf("cannot open the journal file: %v", err))}
	}
	defer journal.Close()
	var got []string
	for _, fact := range journal.Facts() {
		got = append(got, reflect.TypeOf(fact).Name())
	}
	if !slices.Equal(got, l.journalKinds) {
		return []spec.Problem{launchProblem(fmt.Sprintf("expected the journal to hold %v, got %v", l.journalKinds, got))}
	}
	return nil
}
