package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yashikota/enbu/pkg/config"
)

func newSwitchCommand(svc *Service) *cobra.Command {
	var (
		create     bool
		delete     bool
		list       bool
		moveOld    string
		moveNew    string
		doMove     bool
	)

	cmd := &cobra.Command{
		Use:   "switch [env]",
		Short: "Switch, create, or manage environments",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if list {
				return runSwitchList()
			}

			if doMove {
				return runSwitchMove(moveOld, moveNew)
			}

			if delete {
				if len(args) == 0 {
					return fmt.Errorf("environment name required for --delete")
				}
				return runSwitchDelete(args[0])
			}

			if len(args) == 0 {
				return runSwitchList()
			}

			name := args[0]

			if create {
				return runSwitchCreate(name)
			}

			if name == "-" {
				return runSwitchPrevious()
			}

			return runSwitch(name)
		},
	}

	cmd.Flags().BoolVarP(&create, "create", "c", false, "Create a new environment and switch to it")
	cmd.Flags().BoolVarP(&delete, "delete", "d", false, "Delete an environment")
	cmd.Flags().BoolVarP(&list, "list", "l", false, "List all environments")
	cmd.Flags().StringVarP(&moveOld, "move", "m", "", "Rename an environment (old name)")

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if moveOld != "" {
			doMove = true
			if len(args) == 0 {
				return fmt.Errorf("new name required: enbu switch -m <old> <new>")
			}
			moveNew = args[0]
		}
		return nil
	}

	return cmd
}

func runSwitchList() error {
	cfg, err := config.LoadProject()
	if err != nil {
		return err
	}

	names := cfg.EnvironmentNames()
	sort.Strings(names)
	current := cfg.CurrentEnvironment()

	for _, name := range names {
		if name == current {
			fmt.Printf("* %s\n", name)
		} else {
			fmt.Printf("  %s\n", name)
		}
	}
	return nil
}

func runSwitchCreate(name string) error {
	if !config.ValidEnvironmentName(name) {
		return fmt.Errorf("invalid environment name %q", name)
	}

	cfg, err := config.LoadProject()
	if err != nil {
		if strings.Contains(err.Error(), "enbu.toml not found") {
			cfg = config.NewProjectWithEnvironment(name)
			if err := config.SaveProject(cfg); err != nil {
				return err
			}
			fmt.Printf("Created and switched to '%s'\n", name)
			return nil
		}
		return err
	}

	if err := cfg.AddEnvironment(name); err != nil {
		return err
	}

	previous := cfg.CurrentEnvironment()
	cfg.SetDefault(name)

	if err := config.SaveProject(cfg); err != nil {
		return err
	}

	local, _ := config.LoadLocal()
	local.Previous = previous
	_ = config.SaveLocal(local)

	fmt.Printf("Created and switched to '%s'\n", name)
	return nil
}

func runSwitch(name string) error {
	cfg, err := config.LoadProject()
	if err != nil {
		return err
	}

	if !cfg.HasEnvironment(name) {
		return fmt.Errorf("environment %q does not exist (use 'enbu switch -c %s' to create)", name, name)
	}

	previous := cfg.CurrentEnvironment()
	if previous == name {
		fmt.Printf("Already on '%s'\n", name)
		return nil
	}

	cfg.SetDefault(name)

	if err := config.SaveProject(cfg); err != nil {
		return err
	}

	local, _ := config.LoadLocal()
	local.Previous = previous
	_ = config.SaveLocal(local)

	fmt.Printf("Switched to '%s'\n", name)
	return nil
}

func runSwitchPrevious() error {
	local, err := config.LoadLocal()
	if err != nil || local.Previous == "" {
		return fmt.Errorf("no previous environment")
	}

	return runSwitch(local.Previous)
}

func runSwitchDelete(name string) error {
	cfg, err := config.LoadProject()
	if err != nil {
		return err
	}

	if cfg.CurrentEnvironment() == name {
		return fmt.Errorf("cannot delete the current environment '%s' (switch to another first)", name)
	}

	if err := cfg.RemoveEnvironment(name); err != nil {
		return err
	}

	if err := config.SaveProject(cfg); err != nil {
		return err
	}

	fmt.Printf("Deleted environment '%s'\n", name)
	return nil
}

func runSwitchMove(oldName, newName string) error {
	cfg, err := config.LoadProject()
	if err != nil {
		return err
	}

	if err := cfg.RenameEnvironment(oldName, newName); err != nil {
		return err
	}

	if err := config.SaveProject(cfg); err != nil {
		return err
	}

	fmt.Printf("Renamed '%s' to '%s'\n", oldName, newName)
	return nil
}
