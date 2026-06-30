package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/superposition/kotoba-line/internal/sshapp"
)

func main() {
	cfg := sshapp.LoadConfig()

	server, err := sshapp.NewHTTPServer(cfg)
	if err != nil {
		log.Fatalf("configure HTTP server: %v", err)
	}

	errs := make(chan error, 1)
	go func() {
		log.Printf("Kotoba Line web listening on %s", cfg.HTTPAddress())
		errs <- server.ListenAndServe()
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-signals:
		log.Printf("received %s, shutting down", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("shutdown HTTP server: %v", err)
		}
	case err := <-errs:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("run HTTP server: %v", err)
		}
	}
}
