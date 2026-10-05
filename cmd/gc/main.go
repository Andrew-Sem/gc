package main

import (
	"cmp"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/Andrew-Sem/gc/internal/journal"
	"github.com/Andrew-Sem/gc/internal/web"
)

type noGitHub struct{}

func (noGitHub) CanAccess(string) (bool, error) {
	return false, errors.New("проверка доступа в GitHub ещё не сделана")
}

func main() {
	addr := cmp.Or(os.Getenv("GC_ADDR"), "localhost:8080")
	log.Printf("админка: http://%s/repositories", addr)
	log.Fatal(http.ListenAndServe(addr, web.New(&journal.Journal{}, noGitHub{})))
}
