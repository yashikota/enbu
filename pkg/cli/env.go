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
	Name   string
	Output string
}

func addEnvironmentFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVarP(target, "env", "e", "", "Environment to use (overrides current)")
}

func resolveCommandEnvironment(name string) (*commandEnvironment, error) {
	cfg, err := config.LoadProject()
	if err != nil {
		if strings.Contains(err.Error(), "enbu.toml not found") {
			if name == "" {
				name = defaultEnvironment
			}
			return &commandEnvironment{
				Name:   name,
				Output: config.DefaultOutput(name),
			}, nil
		}
		return nil, err
	}

	if name == "" {
		name = cfg.CurrentEnvironment()
	}

	if !config.ValidEnvironmentName(name) {
		return nil, fmt.Errorf("invalid environment %q", name)
	}

	env, err := cfg.Environment(name)
	if err != nil {
		return nil, err
	}
	return &commandEnvironment{
		Name:   name,
		Output: env.Output,
	}, nil
}

func secretsTag(env string) string {
	if env == "" {
		env = defaultEnvironment
	}
	return "secrets-" + oci.CleanTag(env)
}

func recipientTagPrefix() string {
	return "recipient-"
}

func isUserRecipientTag(tag string) bool {
	if tag == "recipient-github-actions" {
		return false
	}
	return strings.HasPrefix(tag, "recipient-")
}
