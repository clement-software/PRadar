package forgejo_test

import (
	"testing"

	"github.com/clement-software/PRadar/internal/adapter/forgejo"
)

func TestParseInstance(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"http://forge.example", "ftp://forge.example", "https://user:pw@forge.example", "https://forge.example/?x=1", "forge.example", ""} {
		if _, err := forgejo.ParseInstance(raw, false); err == nil {
			t.Errorf("ParseInstance(%q) accepted", raw)
		}
	}
	if _, err := forgejo.ParseInstance("http://127.0.0.1:8080", false); err == nil {
		t.Error("loopback HTTP must be opt-in")
	}
	if _, err := forgejo.ParseInstance("http://127.0.0.1:8080", true); err != nil {
		t.Errorf("loopback HTTP rejected for tests: %v", err)
	}
	instance, err := forgejo.ParseInstance("https://forge.example/git/", false)
	if err != nil || instance.String() != "https://forge.example/git" {
		t.Fatalf("ParseInstance = %q, %v", instance.String(), err)
	}
}

func TestParseRepositoryURL(t *testing.T) {
	t.Parallel()
	instance, _ := forgejo.ParseInstance("https://forge.example", false)
	repository, htmlURL, err := instance.ParseRepositoryURL("https://forge.example/acme/widgets.git")
	if err != nil || repository != "acme/widgets" || htmlURL != "https://forge.example/acme/widgets" {
		t.Fatalf("ParseRepositoryURL = %q %q %v", repository, htmlURL, err)
	}
	for _, raw := range []string{
		"https://other.example/acme/widgets", "http://forge.example/acme/widgets", "https://forge.example/acme",
		"https://forge.example/acme/widgets/pulls/1", "https://forge.example/../acme/widgets", "https://forge.example/acme/wid gets",
		"https://user@forge.example/acme/widgets", "https://forge.example/.hidden/widgets",
	} {
		if _, _, err := instance.ParseRepositoryURL(raw); err == nil {
			t.Errorf("ParseRepositoryURL(%q) accepted", raw)
		}
	}
	sub, _ := forgejo.ParseInstance("https://forge.example/git", false)
	if repository, _, err := sub.ParseRepositoryURL("https://forge.example/git/acme/widgets"); err != nil || repository != "acme/widgets" {
		t.Fatalf("sub-path instance: %q %v", repository, err)
	}
	if _, _, err := sub.ParseRepositoryURL("https://forge.example/acme/widgets"); err == nil {
		t.Error("URL outside the instance base path accepted")
	}
}
