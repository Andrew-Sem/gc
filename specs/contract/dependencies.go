package contract

type Journal interface {
	Append(...Fact)
	Facts() []Fact
}

type GitHub struct {
	APIURL string
	Token  string
}

type Dependencies struct {
	Journal Journal
	GitHub  GitHub
}
