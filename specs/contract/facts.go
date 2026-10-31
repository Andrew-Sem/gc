package contract

type Fact interface {
	fact()
}

type RepositoryConnected struct {
	FullName string
}

type RepositoryDisconnected struct {
	FullName string
}

type PrCreated struct {
	Number     int
	Repository string
}

type PrStateChanged struct {
	Number     int
	Repository string
	Open       bool
}

type AdmissionApproved struct {
	Repository string
}

type AdmissionDeclined struct {
	Repository string
	Open       int
	Limit      int
}

type Outcome string

const (
	OutcomePR       Outcome = "PR"
	OutcomeNoPR     Outcome = "no PR"
	OutcomeFailed   Outcome = "failed"
	OutcomeTimedOut Outcome = "timed out"
)

type AgentRunFinished struct {
	Repository string
	Outcome    Outcome
	Number     int
}

func (RepositoryConnected) fact()    {}
func (RepositoryDisconnected) fact() {}
func (PrCreated) fact()              {}
func (PrStateChanged) fact()         {}
func (AdmissionApproved) fact()      {}
func (AdmissionDeclined) fact()      {}
func (AgentRunFinished) fact()       {}
