package policy

import (
	"context"
	"os"
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

// inDir changes cwd to a temp dir, writes enbu.rego, and returns cleanup func.
func inDir(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if content != "" {
		if err := os.WriteFile("enbu.rego", []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoad_FileNotExist(t *testing.T) {
	inDir(t, "")
	eval, err := Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eval != nil {
		t.Fatal("expected nil evaluator for missing file")
	}
}

func TestLoad_InvalidRego(t *testing.T) {
	inDir(t, "this is not valid rego {{{")
	_, err := Load(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid rego")
	}
}

func TestLoad_FindsFileInParentDir(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// write policy in parent, cd into subdir
	if err := os.WriteFile(dir+"/enbu.rego", []byte(testPolicy), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := dir + "/sub"
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}

	eval, err := Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eval == nil {
		t.Fatal("expected evaluator to be found in parent dir")
	}
}

func TestEvaluate_DevAllowsAll(t *testing.T) {
	inDir(t, testPolicy)
	eval, err := Load(context.Background())
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
	inDir(t, testPolicy)
	eval, err := Load(context.Background())
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
	inDir(t, testPolicy)
	eval, err := Load(context.Background())
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
	inDir(t, testPolicy)
	eval, err := Load(context.Background())
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
	inDir(t, testPolicy)
	eval, err := Load(context.Background())
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
	inDir(t, policyContent)
	eval, err := Load(context.Background())
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
