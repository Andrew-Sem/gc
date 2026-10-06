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

type textMismatch struct {
	selector, expected, got string
}

func (p textMismatch) String() string {
	return fmt.Sprintf("expected text %q in %s, got %q", p.expected, p.selector, p.got)
}

type ambiguousElement struct {
	selector string
	got      int
}

func (p ambiguousElement) String() string {
	return fmt.Sprintf("expected one element %s, found %d", p.selector, p.got)
}

func findElement(page *html.Node, selector string) (bool, problem) {
	sel, err := cascadia.Compile(selector)
	if err != nil {
		return false, invalidSelector{selector: selector, err: err.Error()}
	}
	return cascadia.Query(page, sel) != nil, nil
}

func textProblem(page *html.Node, expected textExpectation) problem {
	sel, err := cascadia.Compile(expected.selector)
	if err != nil {
		return invalidSelector{selector: expected.selector, err: err.Error()}
	}
	nodes := cascadia.QueryAll(page, sel)
	if len(nodes) == 0 {
		return elementMissing{selector: expected.selector}
	}
	if len(nodes) > 1 {
		return ambiguousElement{selector: expected.selector, got: len(nodes)}
	}
	got := strings.Join(strings.Fields(textOf(nodes[0])), " ")
	if got != expected.text {
		return textMismatch{selector: expected.selector, expected: expected.text, got: got}
	}
	return nil
}

func textOf(n *html.Node) string {
	var b strings.Builder
	for d := range n.Descendants() {
		if d.Type == html.TextNode {
			b.WriteString(d.Data)
		}
	}
	return b.String()
}

func elementProblems(body string, present, absent []string, texts []textExpectation) []problem {
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
	for _, expected := range texts {
		if p := textProblem(page, expected); p != nil {
			problems = append(problems, p)
		}
	}
	return problems
}
