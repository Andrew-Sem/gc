package contract

type Journal interface {
	Append(...Fact)
	Facts() []Fact
}

type GitHub interface {
	CanAccess(fullName string) (bool, error)
}

type Dependencies struct {
	Journal Journal
	GitHub  GitHub
}
