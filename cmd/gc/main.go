package main

import (
	"cmp"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/Andrew-Sem/gc/internal/agent"
	"github.com/Andrew-Sem/gc/internal/store"
	"github.com/Andrew-Sem/gc/internal/web"
	"github.com/Andrew-Sem/gc/specs/contract"
)

func main() {
	appID := os.Getenv("GITHUB_APP_ID")
	if appID == "" {
		fail("GITHUB_APP_ID is not set")
	}
	keyFile := os.Getenv("GITHUB_APP_PRIVATE_KEY_FILE")
	if keyFile == "" {
		fail("GITHUB_APP_PRIVATE_KEY_FILE is not set")
	}
	privateKey, err := readPrivateKey(keyFile)
	if err != nil {
		fail(fmt.Sprintf("cannot read the private key at %s: %v", keyFile, err))
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
	path := os.Getenv("GC_DB")
	if path == "" {
		fail("GC_DB is not set")
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fail(fmt.Sprintf("cannot listen on %s: %v", addr, err))
	}
	journal, err := store.Open(path, func(err error) {
		fail(fmt.Sprintf("cannot write to the journal at %s: %v", path, err))
	})
	if errors.Is(err, store.ErrUnreadable) {
		fail(fmt.Sprintf("cannot read the journal at %s: %v", path, err))
	}
	if err != nil {
		fail(fmt.Sprintf("cannot open the journal at %s: %v", path, err))
	}
	defer journal.Close()
	github := contract.GitHub{
		APIURL:     cmp.Or(os.Getenv("GITHUB_API_URL"), "https://api.github.com"),
		AppID:      appID,
		PrivateKey: privateKey,
	}
	deps := contract.Dependencies{
		Journal:       journal,
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

func readPrivateKey(path string) (*rsa.PrivateKey, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("the file cannot be read")
	}
	block, _ := pem.Decode(content)
	if block == nil {
		return nil, errors.New("the file is not a PEM private key")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if rsaKey, ok := key.(*rsa.PrivateKey); err == nil && ok {
		return rsaKey, nil
	}
	return nil, errors.New("the file is not an RSA private key")
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
