package web

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/Andrew-Sem/gc/internal/domain"
	"github.com/Andrew-Sem/gc/specs/contract"
)

func Serve(ctx context.Context, listener net.Listener, deps contract.Dependencies) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	log.Printf("next wake-up at %s", nextWakeUp(time.Now()).UTC().Format(time.RFC3339))
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		wakeUpEveryHour(ctx, deps.Journal)
	}()
	server := &http.Server{Handler: New(deps)}
	stop := context.AfterFunc(ctx, func() { server.Close() })
	defer stop()
	err := server.Serve(listener)
	cancel()
	<-stopped
	return err
}

func wakeUpEveryHour(ctx context.Context, journal contract.Journal) {
	for {
		now := time.Now()
		timer := time.NewTimer(nextWakeUp(now).Sub(now))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			domain.WakeUp(journal)
		}
	}
}

func nextWakeUp(now time.Time) time.Time {
	return now.Truncate(time.Hour).Add(time.Hour)
}
