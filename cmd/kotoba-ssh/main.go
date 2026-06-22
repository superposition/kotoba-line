package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	charmssh "github.com/charmbracelet/ssh"
	"github.com/superposition/kotoba-line/internal/sshapp"
)

func main() {
	cfg := sshapp.LoadConfig()

	server, err := sshapp.NewServer(cfg)
	if err != nil {
		log.Fatalf("configure SSH server: %v", err)
	}

	errs := make(chan error, 1)
	go func() {
		log.Printf("Kotoba Line SSH listening on %s", cfg.Address())
		errs <- server.ListenAndServe()
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-signals:
		log.Printf("received %s, shutting down", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil && !errors.Is(err, charmssh.ErrServerClosed) {
			log.Fatalf("shutdown SSH server: %v", err)
		}
	case err := <-errs:
		if err != nil && !errors.Is(err, charmssh.ErrServerClosed) {
			log.Fatalf("run SSH server: %v", err)
		}
	}
}
