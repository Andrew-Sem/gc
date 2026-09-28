package spec

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"golang.org/x/net/html"
)

// selector — небольшое подмножество CSS, которого хватает спекам: тег, #id, .class, [attr],
// [attr="значение"] и потомок через пробел. Полный движок селекторов тут не нужен.
type selector []compound

// compound — одна часть селектора между пробелами, например form[data-spec="connect-form"].
type compound struct {
	tag   string // пусто — любой тег
	attrs []attrMatch
}

type attrMatch struct {
	name  string
	value string
	mode  matchMode
}

type matchMode int

const (
	matchPresent matchMode = iota // [attr]
	matchExact                    // [attr="значение"], #id
	matchWord                     // .class — одно из слов в атрибуте
)

func compileSelector(src string) (selector, error) {
	p := &selectorParser{src: src}
	var sel selector
	for {
		p.skipSpaces()
		if p.done() {
			break
		}
		c, err := p.compound()
		if err != nil {
			return nil, err
		}
		sel = append(sel, c)
	}
	if len(sel) == 0 {
		return nil, errors.New("пустой селектор")
	}
	return sel, nil
}

// matchAll возвращает все элементы документа, подходящие под селектор.
func (sel selector) matchAll(root *html.Node) []*html.Node {
	var found []*html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if sel.matches(n) {
			found = append(found, n)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return found
}

// matches: последняя часть селектора совпадает с самим узлом, остальные — с его предками
// в том же порядке.
func (sel selector) matches(n *html.Node) bool {
	last := len(sel) - 1
	if !sel[last].matches(n) {
		return false
	}
	i := last - 1
	for parent := n.Parent; parent != nil && i >= 0; parent = parent.Parent {
		if sel[i].matches(parent) {
			i--
		}
	}
	return i < 0
}

func (c compound) matches(n *html.Node) bool {
	if n.Type != html.ElementNode || (c.tag != "" && n.Data != c.tag) {
		return false
	}
	for _, a := range c.attrs {
		value, ok := attribute(n, a.name)
		switch {
		case !ok:
			return false
		case a.mode == matchExact && value != a.value:
			return false
		case a.mode == matchWord && !slices.Contains(strings.Fields(value), a.value):
			return false
		}
	}
	return true
}

func attribute(n *html.Node, name string) (string, bool) {
	for _, a := range n.Attr {
		if a.Namespace == "" && a.Key == name {
			return a.Val, true
		}
	}
	return "", false
}

type selectorParser struct {
	src string
	pos int
}

func (p *selectorParser) done() bool { return p.pos >= len(p.src) }

func (p *selectorParser) peek() byte { return p.src[p.pos] }

func (p *selectorParser) skipSpaces() {
	for !p.done() && isSpace(p.peek()) {
		p.pos++
	}
}

func (p *selectorParser) compound() (compound, error) {
	var c compound
	star := false
	switch {
	case p.peek() == '*':
		p.pos++
		star = true
	case isNameByte(p.peek()):
		c.tag = strings.ToLower(p.ident())
	}
	for !p.done() && !isSpace(p.peek()) {
		switch p.peek() {
		case '#', '.':
			mode := matchExact
			name := "id"
			if p.peek() == '.' {
				mode, name = matchWord, "class"
			}
			p.pos++
			value := p.ident()
			if value == "" {
				return c, fmt.Errorf("после %q нет имени (позиция %d)", p.src[p.pos-1], p.pos)
			}
			c.attrs = append(c.attrs, attrMatch{name: name, value: value, mode: mode})
		case '[':
			p.pos++
			a, err := p.attribute()
			if err != nil {
				return c, err
			}
			c.attrs = append(c.attrs, a)
		default:
			return c, fmt.Errorf("непонятный символ %q (позиция %d)", p.peek(), p.pos)
		}
	}
	if c.tag == "" && len(c.attrs) == 0 && !star {
		return c, errors.New("пустая часть селектора")
	}
	return c, nil
}

func (p *selectorParser) attribute() (attrMatch, error) {
	name := strings.ToLower(p.ident())
	if name == "" {
		return attrMatch{}, fmt.Errorf("нет имени атрибута (позиция %d)", p.pos)
	}
	a := attrMatch{name: name, mode: matchPresent}
	if !p.done() && p.peek() == '=' {
		p.pos++
		a.mode = matchExact
		if p.done() {
			return a, errors.New("нет значения атрибута")
		}
		if quote := p.peek(); quote == '"' || quote == '\'' {
			p.pos++
			end := strings.IndexByte(p.src[p.pos:], quote)
			if end < 0 {
				return a, errors.New("незакрытая кавычка")
			}
			a.value = p.src[p.pos : p.pos+end]
			p.pos += end + 1
		} else {
			a.value = p.ident()
		}
	}
	if p.done() || p.peek() != ']' {
		return a, fmt.Errorf("ждали ] после атрибута %s", name)
	}
	p.pos++
	return a, nil
}

func (p *selectorParser) ident() string {
	start := p.pos
	for !p.done() && isNameByte(p.peek()) {
		p.pos++
	}
	return p.src[start:p.pos]
}

func isNameByte(b byte) bool {
	return b == '-' || b == '_' || '0' <= b && b <= '9' || 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z'
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n'
}
