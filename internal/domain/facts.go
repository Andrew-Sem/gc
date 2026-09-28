// Package domain — словарь фактов и правила, по которым они появляются.
package domain

// Fact — запись в журнале: то, что уже произошло.
type Fact interface {
	isFact()
}

// RepositoryConnected — оператор добавил репозиторий, и проверка доступа прошла.
type RepositoryConnected struct {
	Repo string
}

// RepositoryDisconnected — оператор убрал репозиторий. Прошлые факты о нём остаются в журнале.
type RepositoryDisconnected struct {
	Repo string
}

func (RepositoryConnected) isFact()    {}
func (RepositoryDisconnected) isFact() {}
