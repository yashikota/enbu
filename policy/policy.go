package policy

import (
	"context"
	"fmt"
	"os"

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

func Load(path string) (*Evaluator, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading policy file: %w", err)
	}

	prepared, err := rego.New(
		rego.Query("data.enbu.allow_recipient"),
		rego.Module(path, string(data)),
	).PrepareForEval(context.Background())
	if err != nil {
		return nil, fmt.Errorf("compiling policy: %w", err)
	}

	return &Evaluator{prepared: prepared}, nil
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
