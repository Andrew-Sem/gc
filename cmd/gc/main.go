package main

import (
	"cmp"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/Andrew-Sem/gc/framework/journal"
	"github.com/Andrew-Sem/gc/internal/web"
	"github.com/Andrew-Sem/gc/specs/contract"
)

func main() {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fail("GITHUB_TOKEN is not set")
	}
	addr := os.Getenv("GC_ADDR")
	if addr == "" {
		fail("GC_ADDR is not set")
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fail(fmt.Sprintf("cannot listen on %s: %v", addr, err))
	}
	deps := contract.Dependencies{
		Journal: &journal.Journal[contract.Fact]{},
		GitHub: contract.GitHub{
			APIURL: cmp.Or(os.Getenv("GITHUB_API_URL"), "https://api.github.com"),
			Token:  token,
		},
	}
	log.Printf("admin: http://%s/repositories", addr)
	log.Fatal(http.Serve(listener, web.New(deps)))
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
