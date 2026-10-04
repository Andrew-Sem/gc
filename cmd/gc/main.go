package main

import (
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/Andrew-Sem/gc/internal/app"
	"github.com/Andrew-Sem/gc/internal/github"
	"github.com/Andrew-Sem/gc/internal/store"
	"github.com/Andrew-Sem/gc/internal/web"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return errors.New("нужен GITHUB_TOKEN: PAT с правом читать репозитории")
	}
	addr := cmp.Or(os.Getenv("GC_ADDR"), "localhost:8080")
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	a := app.New(app.Deps{
		Journal: store.NewMemory(),
		GitHub:  github.New("https://api.github.com", token, &http.Client{Timeout: 10 * time.Second}),
		Log:     log,
	})
	server := &http.Server{
		Addr:              addr,
		Handler:           web.New(a),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Info("админка запущена", "url", "http://"+addr+"/repositories")
	return server.ListenAndServe()
}
