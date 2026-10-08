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

func (RepositoryConnected) fact()    {}
func (RepositoryDisconnected) fact() {}
