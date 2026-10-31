package main

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/Andrew-Sem/gc/framework/journal"
	"github.com/Andrew-Sem/gc/internal/agent"
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
	if os.Getenv("OPENAI_API_KEY") == "" {
		fail("OPENAI_API_KEY is not set")
	}
	addr := os.Getenv("GC_ADDR")
	if addr == "" {
		fail("GC_ADDR is not set")
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fail(fmt.Sprintf("cannot listen on %s: %v", addr, err))
	}
	github := contract.GitHub{
		APIURL: cmp.Or(os.Getenv("GITHUB_API_URL"), "https://api.github.com"),
		Token:  token,
	}
	deps := contract.Dependencies{
		Journal:       &journal.Journal[contract.Fact]{},
		GitHub:        github,
		WebhookSecret: webhookSecret,
		Agent:         agent.Codex{Binary: "codex", Environ: os.Environ(), GitHub: github},
	}
	log.Printf("admin: http://%s/repositories", addr)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err = web.Serve(ctx, listener, deps)
	if ctx.Err() == nil {
		log.Fatal(err)
	}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
