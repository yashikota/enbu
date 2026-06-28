package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yashikota/enbu/pkg/config"
	"github.com/yashikota/enbu/pkg/oci"
)

const defaultEnvironment = "default"

type commandEnvironment struct {
	Name      string
	Output    string
	KnownEnvs []string
}

func addEnvironmentFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVar(target, "env", defaultEnvironment, "Environment to use")
}

func resolveCommandEnvironment(name string) (*commandEnvironment, error) {
	name = normalizeEnvironmentName(name)
	if !config.ValidEnvironmentName(name) {
		return nil, fmt.Errorf("invalid environment %q", name)
	}

	cfg, err := config.LoadProject()
	if err != nil {
		if name == defaultEnvironment && strings.Contains(err.Error(), "enbu.toml not found") {
			return &commandEnvironment{
				Name:      name,
				Output:    config.DefaultOutput(name),
				KnownEnvs: []string{name},
			}, nil
		}
		return nil, err
	}

	env, err := cfg.Environment(name)
	if err != nil {
		return nil, err
	}
	return &commandEnvironment{
		Name:      name,
		Output:    env.Output,
		KnownEnvs: cfg.EnvironmentNames(),
	}, nil
}

func normalizeEnvironmentName(name string) string {
	if name == "" {
		return defaultEnvironment
	}
	return name
}

func secretsTag(env string) string {
	return "secrets-" + oci.CleanTag(normalizeEnvironmentName(env))
}

func recipientTagPrefix(env string) string {
	if normalizeEnvironmentName(env) == defaultEnvironment {
		return "recipient-"
	}
	return "recipient-" + oci.CleanTag(env) + "-"
}

func isUserRecipientTagForEnv(tag, env string, knownEnvs []string) bool {
	if tag == "recipient-github-actions" {
		return false
	}
	env = normalizeEnvironmentName(env)
	prefix := recipientTagPrefix(env)
	if !strings.HasPrefix(tag, prefix) {
		return false
	}
	for _, known := range knownEnvs {
		known = normalizeEnvironmentName(known)
		if known == env {
			continue
		}
		knownPrefix := recipientTagPrefix(known)
		if len(knownPrefix) > len(prefix) && strings.HasPrefix(tag, knownPrefix) {
			return false
		}
	}
	return true
}
