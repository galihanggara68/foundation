package emit

import (
	"os"
	"path/filepath"
	"runtime"
)

// CloneBoolMap copies a declared-variable tracking set so branch scopes can shadow
// variable declarations without leaking into sibling scopes. Shared by all step emitters.
func CloneBoolMap(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func findContribRuntime(envVar, langDir string) string {
	if env := os.Getenv(envVar); env != "" {
		return env
	}
	candidates := []string{
		filepath.Join("contrib", langDir, "runtime"),
		filepath.Join("..", "contrib", langDir, "runtime"),
		filepath.Join("..", "..", "contrib", langDir, "runtime"),
		filepath.Join("..", "..", "..", "contrib", langDir, "runtime"),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			if abs, err := filepath.Abs(c); err == nil {
				return abs
			}
		}
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if ok {
		repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".."))
		contrib := filepath.Join(repoRoot, "contrib", langDir, "runtime")
		if fi, err := os.Stat(contrib); err == nil && fi.IsDir() {
			return contrib
		}
	}
	return ""
}

// FindContribGoRuntime returns the absolute path to contrib/go/runtime.
func FindContribGoRuntime() string {
	return findContribRuntime("FOUNDATION_CONTRIB_GO_RUNTIME", "go")
}

// FindContribTSRuntime returns the absolute path to contrib/ts/runtime.
func FindContribTSRuntime() string {
	return findContribRuntime("FOUNDATION_CONTRIB_TS_RUNTIME", "ts")
}
