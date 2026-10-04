package domain

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

func IsConnected(facts []Fact, fullName string) bool {
	connected := false
	for _, f := range facts {
		switch f := f.(type) {
		case RepositoryConnected:
			if f.FullName == fullName {
				connected = true
			}
		case RepositoryDisconnected:
			if f.FullName == fullName {
				connected = false
			}
		}
	}
	return connected
}
