package keychain_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clement-software/PRadar/internal/adapter/keychain"
)

// fakeSecurity is a shell substitute for /usr/bin/security backed by one file.
func fakeSecurity(t *testing.T) keychain.Store {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "security")
	store := filepath.Join(dir, "item")
	content := `#!/bin/sh
if [ "$1" = "-i" ]; then
  read -r line
  case "$line" in add-generic-password*) printf '%s' "$line" | sed 's/.*-w "\(.*\)"$/\1/' > "` + store + `";; esac
  exit 0
fi
if [ "$1" = "find-generic-password" ]; then
  if [ -f "` + store + `" ]; then cat "` + store + `"; echo; exit 0; fi
  echo "security: SecKeychainSearchCopyNext: The specified item could not be found in the keychain." >&2
  exit 44
fi
exit 2
`
	if err := os.WriteFile(script, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
	return keychain.Store{Security: script}
}

func TestStore_SaveAndLookup(t *testing.T) {
	t.Parallel()
	store := fakeSecurity(t)
	if _, err := store.Lookup(t.Context(), "forge.example"); !errors.Is(err, keychain.ErrNotFound) {
		t.Fatalf("Lookup(empty) = %v", err)
	}
	if err := store.Save(t.Context(), "forge.example", " "); err == nil {
		t.Fatal("empty token accepted")
	}
	if err := store.Save(t.Context(), "forge.example", "tok_123"); err != nil {
		t.Fatal(err)
	}
	token, err := store.Lookup(t.Context(), "forge.example")
	if err != nil || string(token) != "tok_123" {
		t.Fatalf("Lookup = %q, %v", string(token), err)
	}
	if strings.Contains(token.String(), "tok_123") {
		t.Fatal("token must not print itself")
	}
}
