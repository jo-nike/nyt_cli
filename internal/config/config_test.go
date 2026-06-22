package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAPIKeyPrecedence(t *testing.T) {
	t.Setenv("NYT_API_KEY", "")
	t.Setenv("NYT_KEY", "")

	// Flag wins.
	if k, src, err := ResolveAPIKey("flagkey"); err != nil || k != "flagkey" || src != "--api-key flag" {
		t.Fatalf("flag precedence failed: %q %q %v", k, src, err)
	}

	// Env (NYT_KEY) is used when no flag.
	t.Setenv("NYT_KEY", "envkey")
	if k, _, err := ResolveAPIKey(""); err != nil || k != "envkey" {
		t.Fatalf("env resolution failed: %q %v", k, err)
	}

	// NYT_API_KEY takes priority over NYT_KEY.
	t.Setenv("NYT_API_KEY", "primary")
	if k, _, err := ResolveAPIKey(""); err != nil || k != "primary" {
		t.Fatalf("expected NYT_API_KEY priority, got %q %v", k, err)
	}
}

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "# comment\nexport NYT_KEY=\"abc123\"\nFOO=bar # inline\n"
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NYT_ENV_FILE", envPath)
	t.Setenv("NYT_KEY", "")
	os.Unsetenv("NYT_KEY")
	os.Unsetenv("FOO")

	applyDotEnv(envPath)

	if got := os.Getenv("NYT_KEY"); got != "abc123" {
		t.Fatalf("NYT_KEY = %q, want abc123", got)
	}
	if got := os.Getenv("FOO"); got != "bar" {
		t.Fatalf("FOO = %q, want bar (inline comment stripped)", got)
	}
}

func TestApplyDotEnvDoesNotOverride(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("NYT_KEY=fromfile\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NYT_KEY", "fromenv")
	applyDotEnv(envPath)
	if got := os.Getenv("NYT_KEY"); got != "fromenv" {
		t.Fatalf("dotenv overrode existing env: %q", got)
	}
}
