package spec

import "slices"

type fakeGitHub struct {
	sees []string
}

func (g *fakeGitHub) CanAccess(fullName string) (bool, error) {
	return slices.Contains(g.sees, fullName), nil
}
