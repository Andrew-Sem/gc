package contract

import "time"

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

type Mergeable string

const (
	MergeableYes     Mergeable = "yes"
	MergeableNo      Mergeable = "no"
	MergeableUnknown Mergeable = "unknown"
)

type PrStateChanged struct {
	Number     int
	Repository string
	HeadSHA    string
	Base       string
	Draft      bool
	Open       bool
	Mergeable  Mergeable
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type RunRequested struct {
	Repository string
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

	OutcomeInterrupted Outcome = "interrupted"
)

type AgentRunStarted struct {
	Run        int
	Repository string
	At         time.Time
}

type AgentRunFinished struct {
	Run        int
	Repository string
	Outcome    Outcome
	Number     int
	At         time.Time
}

func (RepositoryConnected) fact()    {}
func (RepositoryDisconnected) fact() {}
func (PrCreated) fact()              {}
func (RunRequested) fact()           {}
func (PrStateChanged) fact()         {}
func (AdmissionApproved) fact()      {}
func (AdmissionDeclined) fact()      {}
func (AgentRunStarted) fact()        {}
func (AgentRunFinished) fact()       {}
