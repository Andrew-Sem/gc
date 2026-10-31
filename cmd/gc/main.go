package main

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"net"
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
	webhookSecret := os.Getenv("GC_WEBHOOK_SECRET")
	if webhookSecret == "" {
		fail("GC_WEBHOOK_SECRET is not set")
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
		WebhookSecret: webhookSecret,
	}
	log.Printf("admin: http://%s/repositories", addr)
	log.Fatal(web.Serve(context.Background(), listener, deps))
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
