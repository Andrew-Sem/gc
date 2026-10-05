package spec

import (
	"fmt"
	"strings"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

type elementMissing struct {
	selector string
}

func (p elementMissing) String() string {
	return fmt.Sprintf("expected element %s, found none", p.selector)
}

type invalidSelector struct {
	selector, err string
}

func (p invalidSelector) String() string {
	return fmt.Sprintf("invalid selector %s: %s", p.selector, p.err)
}

type unreadablePage struct {
	err string
}

func (p unreadablePage) String() string {
	return fmt.Sprintf("cannot read the page: %s", p.err)
}

func elementProblems(body string, selectors []string) []problem {
	page, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return []problem{unreadablePage{err: err.Error()}}
	}
	var problems []problem
	for _, selector := range selectors {
		sel, err := cascadia.Compile(selector)
		if err != nil {
			problems = append(problems, invalidSelector{selector: selector, err: err.Error()})
			continue
		}
		if cascadia.Query(page, sel) == nil {
			problems = append(problems, elementMissing{selector: selector})
		}
	}
	return problems
}
