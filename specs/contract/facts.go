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

type RepositorySettingsChanged struct {
	Repository string
	Pace       int
	Limit      int
	Merge      bool
	At         time.Time
}

type SweepRequested struct {
	Repository string
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

type MergeApproved struct {
	Repository string
	Number     int
	HeadSHA    string
}

type MergeDeclined struct {
	Repository string
	Number     int
	Reason     string
}

type MergeFailed struct {
	Repository string
	Number     int
	Reason     string
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
	Run         int
	Repository  string
	Outcome     Outcome
	Number      int
	Explanation string
	Tokens      TokenUsage
	At          time.Time
}

func (RepositoryConnected) fact()       {}
func (RepositoryDisconnected) fact()    {}
func (PrCreated) fact()                 {}
func (RunRequested) fact()              {}
func (RepositorySettingsChanged) fact() {}
func (PrStateChanged) fact()            {}
func (AdmissionApproved) fact()         {}
func (AdmissionDeclined) fact()         {}
func (AgentRunStarted) fact()           {}
func (AgentRunFinished) fact()          {}
func (MergeApproved) fact()             {}
func (MergeDeclined) fact()             {}
func (MergeFailed) fact()               {}
func (SweepRequested) fact()            {}
