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

type unexpectedElement struct {
	selector string
}

func (p unexpectedElement) String() string {
	return fmt.Sprintf("expected no element %s, found one", p.selector)
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

func findElement(page *html.Node, selector string) (bool, problem) {
	sel, err := cascadia.Compile(selector)
	if err != nil {
		return false, invalidSelector{selector: selector, err: err.Error()}
	}
	return cascadia.Query(page, sel) != nil, nil
}

func elementProblems(body string, present, absent []string) []problem {
	page, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return []problem{unreadablePage{err: err.Error()}}
	}
	var problems []problem
	for _, selector := range present {
		found, p := findElement(page, selector)
		if p != nil {
			problems = append(problems, p)
		} else if !found {
			problems = append(problems, elementMissing{selector: selector})
		}
	}
	for _, selector := range absent {
		found, p := findElement(page, selector)
		if p != nil {
			problems = append(problems, p)
		} else if found {
			problems = append(problems, unexpectedElement{selector: selector})
		}
	}
	return problems
}
