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
}

type AdmissionApproved struct {
	Repository string
}

func (RepositoryConnected) fact()    {}
func (RepositoryDisconnected) fact() {}
func (PrCreated) fact()              {}
func (PrStateChanged) fact()         {}
func (AdmissionApproved) fact()      {}
