// Package forgejo is the read-only HTTP adapter for the single configured
// Forgejo instance: URL mapping, authentication, pagination and payload
// decoding behind the collection contracts declared by internal/app.
package forgejo

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

var namePart = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9._-]{0,99})$`)

// Instance is the validated origin of the configured Forgejo instance.
type Instance struct{ base *url.URL }

// ParseInstance accepts an HTTPS origin, or a loopback HTTP origin when
// allowLoopbackHTTP is set for automated tests.
func ParseInstance(raw string, allowLoopbackHTTP bool) (Instance, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return Instance{}, fmt.Errorf("invalid instance URL: %w", err)
	}
	host := parsed.Hostname()
	switch {
	case parsed.Host == "" || host == "":
		return Instance{}, errors.New("instance URL must include a host")
	case parsed.User != nil:
		return Instance{}, errors.New("instance URL must not embed credentials")
	case parsed.RawQuery != "" || parsed.Fragment != "":
		return Instance{}, errors.New("instance URL must not include a query or fragment")
	case parsed.Scheme == "https":
	case parsed.Scheme == "http" && allowLoopbackHTTP && isLoopback(host):
	default:
		return Instance{}, fmt.Errorf("instance URL scheme %q is not supported; use https", parsed.Scheme)
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	parsed.RawPath = ""
	return Instance{base: parsed}, nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// String is the canonical origin plus optional base path.
func (i Instance) String() string { return i.base.String() }

// Origin is scheme://host used for redirect checks.
func (i Instance) Origin() string { return i.base.Scheme + "://" + i.base.Host }

// ParseRepositoryURL maps a normal Forgejo repository URL of this instance to
// "owner/name" and its canonical URL. Cross-origin, malformed or ambiguous
// URLs are rejected before any credential is sent.
func (i Instance) ParseRepositoryURL(raw string) (repository, htmlURL string, err error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", "", fmt.Errorf("invalid repository URL: %w", err)
	}
	if parsed.Scheme != i.base.Scheme || !strings.EqualFold(parsed.Host, i.base.Host) || parsed.User != nil {
		return "", "", fmt.Errorf("repository URL must belong to %s", i.Origin())
	}
	rest, ok := strings.CutPrefix(parsed.Path, i.base.Path+"/")
	if !ok {
		return "", "", fmt.Errorf("repository URL must be below %s", i.String())
	}
	rest = strings.TrimSuffix(strings.TrimSuffix(rest, "/"), ".git")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || !namePart.MatchString(parts[0]) || !namePart.MatchString(parts[1]) {
		return "", "", errors.New("repository URL must have the form <instance>/<owner>/<name>")
	}
	repository = parts[0] + "/" + parts[1]
	return repository, i.String() + "/" + repository, nil
}
