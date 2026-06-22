package sshapp

import (
	"context"
	"errors"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	charmssh "github.com/charmbracelet/ssh"
	gossh "golang.org/x/crypto/ssh"
)

func TestServerRejectsWrongPassword(t *testing.T) {
	addr := startTestServer(t)

	client, err := dialTestServer(addr, "player", "wrong")
	if err == nil {
		client.Close()
		t.Fatal("Dial succeeded with wrong password")
	}
}

func TestServerRendersTUIOverPTY(t *testing.T) {
	addr := startTestServer(t)

	client, err := dialTestServer(addr, "player", "kotoba")
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	if err := session.RequestPty("xterm-256color", 24, 80, gossh.TerminalModes{}); err != nil {
		t.Fatalf("request PTY: %v", err)
	}
	if err := session.Shell(); err != nil {
		t.Fatalf("start shell session: %v", err)
	}

	output := waitForOutput(t, stdout, "Kotoba Line")
	if !strings.Contains(output, "Player: player") {
		t.Fatalf("TUI output missing player:\n%s", output)
	}

	if _, err := stdin.Write([]byte("q")); err != nil {
		t.Fatalf("send quit key: %v", err)
	}
	waitForSession(t, session)
}

func TestServerRejectsRemoteCommand(t *testing.T) {
	addr := startTestServer(t)

	client, err := dialTestServer(addr, "player", "kotoba")
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput("echo hi")
	if err == nil {
		t.Fatal("remote command succeeded, want rejection")
	}
	if !strings.Contains(string(output), "does not expose a shell or command runner") {
		t.Fatalf("remote command output = %q", string(output))
	}
}

func startTestServer(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	cfg := Config{
		Host:        "127.0.0.1",
		Port:        port,
		User:        "player",
		Password:    "kotoba",
		HostKeyPath: filepath.Join(t.TempDir(), "ssh_host_ed25519"),
	}
	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	errs := make(chan error, 1)
	go func() {
		errs <- server.Serve(listener)
	}()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil && !errors.Is(err, charmssh.ErrServerClosed) {
			t.Errorf("shutdown server: %v", err)
		}
		select {
		case err := <-errs:
			if err != nil && !errors.Is(err, charmssh.ErrServerClosed) {
				t.Errorf("serve: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	})

	return listener.Addr().String()
}

func dialTestServer(addr string, user string, password string) (*gossh.Client, error) {
	return gossh.Dial("tcp", addr, &gossh.ClientConfig{
		User:            user,
		Auth:            []gossh.AuthMethod{gossh.Password(password)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         2 * time.Second,
	})
}

func waitForOutput(t *testing.T, reader io.Reader, want string) string {
	t.Helper()

	chunks := make(chan string, 16)
	errs := make(chan error, 1)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				chunks <- string(buf[:n])
			}
			if err != nil {
				errs <- err
				return
			}
		}
	}()

	var output strings.Builder
	timeout := time.After(3 * time.Second)
	for {
		select {
		case chunk := <-chunks:
			output.WriteString(chunk)
			if strings.Contains(output.String(), want) {
				return output.String()
			}
		case err := <-errs:
			t.Fatalf("read session output before %q: %v\n%s", want, err, output.String())
		case <-timeout:
			t.Fatalf("timed out waiting for %q:\n%s", want, output.String())
		}
	}
}

func waitForSession(t *testing.T, session *gossh.Session) {
	t.Helper()

	done := make(chan error, 1)
	go func() {
		done <- session.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("wait session: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("session did not exit after quit key")
	}
}
