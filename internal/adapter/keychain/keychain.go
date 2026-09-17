// Package keychain stores and looks up the Forgejo token in the macOS
// Keychain through the system `security` tool, so the credential never lives
// in SQLite, configuration files, logs or the repository.
package keychain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/clement-software/PRadar/internal/adapter/forgejo"
)

// Service is the Keychain service name of PRadar items.
const Service = "pradar.forgejo"

// ErrNotFound is returned when no token is stored for the account.
var ErrNotFound = errors.New("no Forgejo token in the Keychain; run `pradar token set`")

// Store wraps the `security` executable.
type Store struct {
	// Security is the path of the security tool; defaults to /usr/bin/security.
	Security string
}

func (s Store) security() string {
	if s.Security == "" {
		return "/usr/bin/security"
	}
	return s.Security
}

// Lookup reads the token stored for the instance host.
func (s Store) Lookup(ctx context.Context, account string) (forgejo.Token, error) {
	cmd := exec.CommandContext(ctx, s.security(), "find-generic-password", "-s", Service, "-a", account, "-w") //nolint:gosec // fixed executable, arguments are not shell-interpreted
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if strings.Contains(stderr.String(), "could not be found") || (errors.As(err, &exit) && exit.ExitCode() == 44) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("keychain lookup failed: %w", err) // stderr never echoed: it may quote the item
	}
	token := forgejo.Token(strings.TrimRight(stdout.String(), "\r\n"))
	if token == "" {
		return "", ErrNotFound
	}
	return token, nil
}

// Save stores or replaces the token for the instance host. The token travels
// on standard input rather than the argument list.
func (s Store) Save(ctx context.Context, account string, token forgejo.Token) error {
	if strings.TrimSpace(string(token)) == "" {
		return errors.New("token is empty")
	}
	cmd := exec.CommandContext(ctx, s.security(), "-i") //nolint:gosec // fixed executable; the token travels on stdin
	cmd.Stdin = strings.NewReader(fmt.Sprintf("add-generic-password -U -s %q -a %q -w %q\n", Service, account, string(token)))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("keychain save failed: %w: %s", err, token.String()+strings.ReplaceAll(string(output), string(token), "[redacted]"))
	}
	return nil
}
