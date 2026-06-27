package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureGitignore_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()

	if err := ensureGitignore(dir); err != nil {
		t.Fatalf("ensureGitignore: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	content := string(data)
	for _, entry := range gitignoreEntries {
		if !strings.Contains(content, entry) {
			t.Errorf("missing entry %q in .gitignore", entry)
		}
	}
}

func TestEnsureGitignore_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	existing := "node_modules/\n*.log\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ensureGitignore(dir); err != nil {
		t.Fatalf("ensureGitignore: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.HasPrefix(content, existing) {
		t.Error("existing content was overwritten")
	}
	for _, entry := range gitignoreEntries {
		if !strings.Contains(content, entry) {
			t.Errorf("missing entry %q", entry)
		}
	}
}

func TestEnsureGitignore_NoDuplicates(t *testing.T) {
	dir := t.TempDir()
	existing := ".env\n.env.*\n!.env.example\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ensureGitignore(dir); err != nil {
		t.Fatalf("ensureGitignore: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != existing {
		t.Errorf("file was modified when all entries already exist:\ngot: %q\nwant: %q", string(data), existing)
	}
}

func TestEnsureGitignore_PartialExisting(t *testing.T) {
	dir := t.TempDir()
	existing := ".env\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ensureGitignore(dir); err != nil {
		t.Fatalf("ensureGitignore: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, ".env.*") {
		t.Error("missing .env.*")
	}
	if !strings.Contains(content, "!.env.example") {
		t.Error("missing !.env.example")
	}
	if strings.Count(content, ".env\n") != 1 {
		t.Error(".env was duplicated")
	}
}

func TestEnsureGitignore_NoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	existing := "node_modules/"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ensureGitignore(dir); err != nil {
		t.Fatalf("ensureGitignore: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.HasPrefix(content, "node_modules/\n") {
		t.Error("newline not added after existing content without trailing newline")
	}
	for _, entry := range gitignoreEntries {
		if !strings.Contains(content, entry) {
			t.Errorf("missing entry %q", entry)
		}
	}
}

func TestCleanTag(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"user-abc123", "user-abc123"},
		{"user@host", "user-host"},
		{"a/b/c", "a-b-c"},
		{strings.Repeat("x", 200), strings.Repeat("x", 128)},
	}

	for _, tt := range tests {
		got := cleanTag(tt.input)
		if got != tt.want {
			t.Errorf("cleanTag(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestKeyFingerprint(t *testing.T) {
	fp := keyFingerprint("age1abc123")
	if len(fp) != 8 {
		t.Errorf("fingerprint length = %d, want 8", len(fp))
	}

	fp2 := keyFingerprint("age1abc123")
	if fp != fp2 {
		t.Error("fingerprint not deterministic")
	}

	fp3 := keyFingerprint("age1different")
	if fp == fp3 {
		t.Error("different keys produced same fingerprint")
	}
}

func TestIsUserRecipientTag(t *testing.T) {
	tests := []struct {
		tag  string
		want bool
	}{
		{"recipient-alice-12345678", true},
		{"recipient-github-actions", false},
		{"secrets-default", false},
		{"user-alice", false},
	}

	for _, tt := range tests {
		if got := isUserRecipientTag(tt.tag); got != tt.want {
			t.Errorf("isUserRecipientTag(%q) = %v, want %v", tt.tag, got, tt.want)
		}
	}
}

func TestGitCommitInitFiles_NoGitRepo(t *testing.T) {
	dir := t.TempDir()
	err := gitCommitInitFiles(dir)
	if err == nil {
		t.Fatal("expected error for non-git directory")
	}
}
