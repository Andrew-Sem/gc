package spec

import (
	"errors"
	"slices"
)

type fakeGitHub struct {
	sees          []string
	asked         []string
	doesNotAnswer bool
}

func (g *fakeGitHub) CanAccess(fullName string) (bool, error) {
	g.asked = append(g.asked, fullName)
	if g.doesNotAnswer {
		return false, errors.New("GitHub does not answer")
	}
	return slices.Contains(g.sees, fullName), nil
}
