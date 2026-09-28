package spec

import (
	"fmt"
	"regexp"
	"strings"
)

// contract — пункты договора фичи в порядке их появления в файле.
type contract struct {
	file    string
	clauses []clause
}

type clause struct {
	id   string
	line int
	text string
}

// clauseStart — первая строка пункта вида «REPO-1.2. Текст пункта».
var clauseStart = regexp.MustCompile(`^([A-Z]+-\d+(?:\.\d+)*)\.(?:\s+(.*))?$`)

// parseContract читает пункты договора. Пункт продолжается до пустой строки или заголовка;
// строки внутри блоков кода ``` пунктами не считаются.
func parseContract(file string, data []byte) (contract, error) {
	c := contract{file: file}
	seen := map[string]int{}
	inCode := false
	current := -1
	for i, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```"):
			inCode = !inCode
			current = -1
		case inCode:
		case clauseStart.MatchString(line):
			m := clauseStart.FindStringSubmatch(line)
			id := m[1]
			if first, dup := seen[id]; dup {
				return contract{}, fmt.Errorf("пункт %s повторяется: строки %d и %d", id, first, i+1)
			}
			seen[id] = i + 1
			c.clauses = append(c.clauses, clause{id: id, line: i + 1, text: strings.TrimSpace(m[2])})
			current = len(c.clauses) - 1
		case trimmed == "" || strings.HasPrefix(trimmed, "#"):
			current = -1
		case current >= 0:
			c.clauses[current].text += " " + trimmed
		}
	}
	if len(c.clauses) == 0 {
		return contract{}, fmt.Errorf("в договоре нет ни одного пункта")
	}
	for i := range c.clauses {
		c.clauses[i].text = strings.TrimSpace(c.clauses[i].text)
		if c.clauses[i].text == "" {
			return contract{}, fmt.Errorf("у пункта %s нет текста (строка %d)", c.clauses[i].id, c.clauses[i].line)
		}
	}
	return c, nil
}

func (c contract) has(id string) bool {
	for _, cl := range c.clauses {
		if cl.id == id {
			return true
		}
	}
	return false
}

// uncovered — пункты, на которые не ссылается ни один сценарий, в порядке договора.
func (c contract) uncovered(covered map[string]bool) []string {
	var missing []string
	for _, cl := range c.clauses {
		if !covered[cl.id] {
			missing = append(missing, cl.id)
		}
	}
	return missing
}
