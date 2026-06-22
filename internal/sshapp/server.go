package sshapp

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	charmssh "github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	wishtea "github.com/charmbracelet/wish/bubbletea"
	"github.com/superposition/kotoba-line/internal/tui/app"
)

func NewServer(cfg Config) (*charmssh.Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := ensureHostKeyDir(cfg.HostKeyPath); err != nil {
		return nil, err
	}

	auth := NewAuthenticator(cfg.User, cfg.Password)

	return wish.NewServer(
		wish.WithAddress(cfg.Address()),
		wish.WithHostKeyPath(cfg.HostKeyPath),
		wish.WithPasswordAuth(func(ctx charmssh.Context, password string) bool {
			return auth.Authenticate(ctx.User(), password)
		}),
		wish.WithMiddleware(
			wishtea.Middleware(sessionModel),
		),
	)
}

func sessionModel(sess charmssh.Session) (tea.Model, []tea.ProgramOption) {
	if strings.TrimSpace(sess.RawCommand()) != "" {
		wish.Fatalln(sess, "Kotoba Line does not expose a shell or command runner.")
		return nil, nil
	}

	return app.New(app.Options{Username: sess.User()}), []tea.ProgramOption{
		tea.WithAltScreen(),
	}
}

func ensureHostKeyDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o700)
}
