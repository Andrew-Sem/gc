package spec

import "slices"

type fakeGitHub struct {
	sees  []string
	asked []string
}

func (g *fakeGitHub) CanAccess(fullName string) (bool, error) {
	g.asked = append(g.asked, fullName)
	return slices.Contains(g.sees, fullName), nil
}
