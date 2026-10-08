package main

import (
	"cmp"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/Andrew-Sem/gc/framework/journal"
	"github.com/Andrew-Sem/gc/internal/web"
	"github.com/Andrew-Sem/gc/specs/contract"
)

type noGitHub struct{}

func (noGitHub) CanAccess(string) (bool, error) {
	return false, errors.New("GitHub access check is not implemented yet")
}

func main() {
	addr := cmp.Or(os.Getenv("GC_ADDR"), "localhost:8080")
	log.Printf("admin: http://%s/repositories", addr)
	log.Fatal(http.ListenAndServe(addr, web.New(contract.Dependencies{Journal: &journal.Journal[contract.Fact]{}, GitHub: noGitHub{}})))
}
