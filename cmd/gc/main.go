package main

import (
	"cmp"
	"log"
	"net/http"
	"os"

	"github.com/Andrew-Sem/gc/framework/journal"
	"github.com/Andrew-Sem/gc/internal/web"
	"github.com/Andrew-Sem/gc/specs/contract"
)

func main() {
	addr := cmp.Or(os.Getenv("GC_ADDR"), "localhost:8080")
	deps := contract.Dependencies{
		Journal: &journal.Journal[contract.Fact]{},
		GitHub: contract.GitHub{
			APIURL: cmp.Or(os.Getenv("GITHUB_API_URL"), "https://api.github.com"),
			Token:  os.Getenv("GITHUB_TOKEN"),
		},
	}
	log.Printf("admin: http://%s/repositories", addr)
	log.Fatal(http.ListenAndServe(addr, web.New(deps)))
}
