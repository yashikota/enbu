package policy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

const testPolicy = `package enbu

import rego.v1

default allow_recipient := false

allow_recipient if {
	input.target_env == "dev"
}

allow_recipient if {
	input.target_env == "staging"
	"backend" in input.recipient.teams
}

allow_recipient if {
	input.target_env == "production"
	"infra" in input.recipient.teams
}
`

func writePolicy(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "enbu.rego")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_FileNotExist(t *testing.T) {
	eval, err := Load("/nonexistent/enbu.rego")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eval != nil {
		t.Fatal("expected nil evaluator for missing file")
	}
}

func TestLoad_InvalidRego(t *testing.T) {
	path := writePolicy(t, "this is not valid rego {{{")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid rego")
	}
}

func TestEvaluate_DevAllowsAll(t *testing.T) {
	path := writePolicy(t, testPolicy)
	eval, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	input := &Input{
		TargetEnv: "dev",
		Recipient: RecipientInput{Username: "anyone", Teams: []string{}},
		Repo:      RepoInput{Owner: "org", Name: "repo", IsOrg: true},
	}
	allowed, err := eval.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !allowed {
		t.Fatal("expected dev to allow all")
	}
}

func TestEvaluate_StagingAllowsBackendTeam(t *testing.T) {
	path := writePolicy(t, testPolicy)
	eval, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	input := &Input{
		TargetEnv: "staging",
		Recipient: RecipientInput{Username: "charlie", Teams: []string{"backend"}},
		Repo:      RepoInput{Owner: "org", Name: "repo", IsOrg: true},
	}
	allowed, err := eval.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !allowed {
		t.Fatal("expected staging to allow backend team")
	}
}

func TestEvaluate_StagingDeniesNonBackend(t *testing.T) {
	path := writePolicy(t, testPolicy)
	eval, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	input := &Input{
		TargetEnv: "staging",
		Recipient: RecipientInput{Username: "outsider", Teams: []string{"design"}},
		Repo:      RepoInput{Owner: "org", Name: "repo", IsOrg: true},
	}
	allowed, err := eval.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if allowed {
		t.Fatal("expected staging to deny non-backend team")
	}
}

func TestEvaluate_ProductionAllowsInfra(t *testing.T) {
	path := writePolicy(t, testPolicy)
	eval, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	input := &Input{
		TargetEnv: "production",
		Recipient: RecipientInput{Username: "alice", Teams: []string{"infra", "backend"}},
		Repo:      RepoInput{Owner: "org", Name: "repo", IsOrg: true},
	}
	allowed, err := eval.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !allowed {
		t.Fatal("expected production to allow infra team")
	}
}

func TestEvaluate_ProductionDeniesBackendOnly(t *testing.T) {
	path := writePolicy(t, testPolicy)
	eval, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	input := &Input{
		TargetEnv: "production",
		Recipient: RecipientInput{Username: "charlie", Teams: []string{"backend"}},
		Repo:      RepoInput{Owner: "org", Name: "repo", IsOrg: true},
	}
	allowed, err := eval.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if allowed {
		t.Fatal("expected production to deny backend-only team")
	}
}

func TestEvaluate_PermissionBasedPolicy(t *testing.T) {
	policyContent := `package enbu

import rego.v1

default allow_recipient := false

allow_recipient if {
	input.target_env == "production"
	input.recipient.permission == "admin"
}
`
	path := writePolicy(t, policyContent)
	eval, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	tests := []struct {
		name       string
		permission string
		want       bool
	}{
		{"admin allowed", "admin", true},
		{"write denied", "write", false},
		{"read denied", "read", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &Input{
				TargetEnv: "production",
				Recipient: RecipientInput{Username: "user", Permission: tt.permission},
				Repo:      RepoInput{Owner: "alice", Name: "repo", IsOrg: false},
			}
			allowed, err := eval.Evaluate(context.Background(), input)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if allowed != tt.want {
				t.Fatalf("got %v, want %v", allowed, tt.want)
			}
		})
	}
}
