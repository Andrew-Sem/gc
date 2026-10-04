package domain

type Fact interface {
	isFact()
}

type RepositoryConnected struct {
	Repo string
}

type RepositoryDisconnected struct {
	Repo string
}

func (RepositoryConnected) isFact()    {}
func (RepositoryDisconnected) isFact() {}
