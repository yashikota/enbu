package policy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-policy-agent/opa/v1/rego"
)

type Input struct {
	TargetEnv string         `json:"target_env"`
	Recipient RecipientInput `json:"recipient"`
	Repo      RepoInput      `json:"repo"`
}

type RecipientInput struct {
	Username   string   `json:"username"`
	Teams      []string `json:"teams"`
	Permission string   `json:"permission"`
}

type RepoInput struct {
	Owner string `json:"owner"`
	Name  string `json:"name"`
	IsOrg bool   `json:"is_org"`
}

type Evaluator struct {
	prepared rego.PreparedEvalQuery
}

// Load searches for enbu.rego by walking up from cwd, mirroring how enbu.toml is found.
// Returns nil without error if no policy file exists anywhere in the tree.
func Load(ctx context.Context) (*Evaluator, error) {
	path, err := findPolicyFile()
	if err != nil {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading policy file: %w", err)
	}

	prepared, err := rego.New(
		rego.Query("data.enbu.allow_recipient"),
		rego.Module(path, string(data)),
	).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("compiling policy: %w", err)
	}

	return &Evaluator{prepared: prepared}, nil
}

func findPolicyFile() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		path := filepath.Join(dir, "enbu.rego")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("enbu.rego not found")
}

func (e *Evaluator) Evaluate(ctx context.Context, input *Input) (bool, error) {
	results, err := e.prepared.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return false, fmt.Errorf("evaluating policy: %w", err)
	}

	if len(results) == 0 || len(results[0].Expressions) == 0 {
		return false, nil
	}

	allowed, ok := results[0].Expressions[0].Value.(bool)
	if !ok {
		return false, nil
	}
	return allowed, nil
}
