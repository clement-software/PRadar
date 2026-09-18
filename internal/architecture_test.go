package internal_test

import (
	"go/build"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestArchitecture_DependenciesPointInward enforces ADR-0001: domain and
// application policy import no SQL, HTTP, process, Keychain, UI or adapter code.
func TestArchitecture_DependenciesPointInward(t *testing.T) {
	t.Parallel()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Dir(file)
	forbidden := []string{
		"database/sql", "net/http", "os/exec", "html/template", "net/",
		"github.com/clement-software/PRadar/internal/adapter/",
		"github.com/clement-software/PRadar/internal/ui",
		"modernc.org/", "github.com/mattn/", "github.com/yuin/",
	}
	for _, policy := range []string{"pullrequest", "collect", "analyse", "timeline", "evaluation"} {
		pkg, err := build.ImportDir(filepath.Join(root, policy), 0)
		if err != nil {
			t.Fatalf("import %s: %v", policy, err)
		}
		for _, imported := range pkg.Imports {
			for _, banned := range forbidden {
				if strings.HasPrefix(imported, banned) {
					t.Errorf("%s imports %s, which points outward", policy, imported)
				}
			}
		}
	}
}
