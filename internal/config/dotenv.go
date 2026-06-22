package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var dotenvOnce sync.Once

// LoadDotEnv loads variables from a .env file into the process environment
// (without overriding values already set). It searches the current directory
// and its ancestors, plus an explicit path in $NYT_ENV_FILE. It runs at most
// once per process. Errors are intentionally ignored — a missing .env is normal.
func LoadDotEnv() {
	dotenvOnce.Do(func() {
		if p := os.Getenv("NYT_ENV_FILE"); p != "" {
			applyDotEnv(p)
		}
		if path := findDotEnv(); path != "" {
			applyDotEnv(path)
		}
	})
}

// findDotEnv walks up from the working directory looking for a .env file.
func findDotEnv() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		candidate := filepath.Join(dir, ".env")
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func applyDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		val = unquote(val)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	// Strip trailing inline comments for unquoted values.
	if i := strings.Index(s, " #"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}
