package sshapp

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	charmssh "github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	wishtea "github.com/charmbracelet/wish/bubbletea"
	"github.com/superposition/kotoba-line/internal/content"
	statestore "github.com/superposition/kotoba-line/internal/state"
	"github.com/superposition/kotoba-line/internal/tui/app"
)

func NewServer(cfg Config) (*charmssh.Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := ensureHostKeyDir(cfg.HostKeyPath); err != nil {
		return nil, err
	}

	auth := cfg.Authenticator()

	return wish.NewServer(
		wish.WithAddress(cfg.Address()),
		wish.WithHostKeyPath(cfg.HostKeyPath),
		wish.WithPasswordAuth(func(ctx charmssh.Context, password string) bool {
			return auth.Authenticate(ctx.User(), password)
		}),
		wish.WithMiddleware(
			wishtea.Middleware(func(sess charmssh.Session) (tea.Model, []tea.ProgramOption) {
				return sessionModel(sess, cfg)
			}),
		),
	)
}

func sessionModel(sess charmssh.Session, cfg Config) (tea.Model, []tea.ProgramOption) {
	if strings.TrimSpace(sess.RawCommand()) != "" {
		wish.Fatalln(sess, "Kotoba Beach does not expose a shell or command runner.")
		return nil, nil
	}

	model := NewGameplayModel(sess.User(), cfg, func(message string) {
		wish.Println(sess, message)
	})

	return model, []tea.ProgramOption{
		tea.WithAltScreen(),
	}
}

func NewGameplayModel(username string, cfg Config, warn func(string)) app.Model {
	if warn == nil {
		warn = func(string) {}
	}

	store := statestore.NewSQLiteEventStore(cfg.StateDBPath, username)
	var eventStore statestore.EventStore = store
	var stateSeeder interface {
		statestore.EventStore
		statestore.EventCounter
	} = store
	var touchUser interface{ TouchUser() error } = store
	var library = loadSQLiteLessonLibrary(cfg.StateDBPath, warn)

	if strings.TrimSpace(cfg.DatabaseURL) != "" {
		postgres := statestore.NewPostgresEventStore(cfg.DatabaseURL, username)
		eventStore = postgres
		stateSeeder = postgres
		touchUser = postgres
		loaded, report := statestore.DefaultLessonLibrary()
		if report.HasErrors() {
			warn("lesson load warning: built-in lessons have validation errors")
			library = nil
		} else {
			library = mergeDefaultPlayableLibrary(loaded, warn)
		}
	}

	if _, err := statestore.SeedEventStoreFromEventLogIfEmpty(stateSeeder, statestore.DefaultEventLog()); err != nil {
		warn("state seed warning: " + err.Error())
	}
	if err := touchUser.TouchUser(); err != nil {
		warn("state warning: " + err.Error())
	}

	return app.New(app.Options{Username: username, EventStore: eventStore, Library: library})
}

func loadSQLiteLessonLibrary(stateDBPath string, warn func(string)) *content.Library {
	if _, err := statestore.SeedDefaultLessons(stateDBPath); err != nil {
		warn("lesson seed warning: " + err.Error())
	}
	library, report, err := statestore.LoadLessonLibrary(stateDBPath)
	if err != nil {
		warn("lesson load warning: " + err.Error())
		return nil
	}
	if report.HasErrors() {
		warn("lesson load warning: sqlite lessons have validation errors")
		return nil
	}
	return mergeDefaultPlayableLibrary(library, warn)
}

func mergeDefaultPlayableLibrary(base *content.Library, warn func(string)) *content.Library {
	playable, report, err := content.LoadDefaultPlayableLibrary()
	if err != nil {
		warn("content load warning: " + err.Error())
		return base
	}
	if report.HasErrors() {
		warn("content load warning: default playable content has validation errors")
		return base
	}

	merged := content.MergeLibraries(base, playable)
	report = content.ValidateLibrary(merged)
	if report.HasErrors() {
		warn("content load warning: merged lesson content has validation errors")
		return base
	}
	return merged
}

func ensureHostKeyDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o700)
}
