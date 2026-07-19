package cli

import (
	"fmt"
	"sort"

	"github.com/enbu-net/enbu/app"
	"github.com/spf13/cobra"
)

func newSwitchCommand(a *app.App) *cobra.Command {
	var (
		create  bool
		delete  bool
		list    bool
		moveOld string
		moveNew string
		doMove  bool
	)

	cmd := &cobra.Command{
		Use:   "switch [env]",
		Short: "Switch, create, or manage environments",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if list {
				return runSwitchList(a)
			}

			if doMove {
				if err := a.RenameEnvironment(moveOld, moveNew); err != nil {
					return err
				}
				fmt.Printf("Renamed '%s' to '%s'\n", moveOld, moveNew)
				return nil
			}

			if delete {
				if len(args) == 0 {
					return fmt.Errorf("environment name required for --delete")
				}
				if err := a.DeleteEnvironment(args[0]); err != nil {
					return err
				}
				fmt.Printf("Deleted environment '%s'\n", args[0])
				return nil
			}

			if len(args) == 0 {
				return runSwitchList(a)
			}

			name := args[0]

			if create {
				if err := a.CreateEnvironment(name); err != nil {
					return err
				}
				fmt.Printf("Created and switched to '%s'\n", name)
				return nil
			}

			if name == "-" {
				target, err := a.SwitchPrevious()
				if err != nil {
					return err
				}
				fmt.Printf("Switched to '%s'\n", target)
				return nil
			}

			if err := a.SwitchEnvironment(name); err != nil {
				return err
			}
			fmt.Printf("Switched to '%s'\n", name)
			return nil
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

func runSwitchList(a *app.App) error {
	envs, err := a.ListEnvironments()
	if err != nil {
		return err
	}

	sort.Slice(envs, func(i, j int) bool { return envs[i].Name < envs[j].Name })

	for _, env := range envs {
		if env.IsCurrent {
			fmt.Printf("* %s\n", env.Name)
		} else {
			fmt.Printf("  %s\n", env.Name)
		}
	}
	return nil
}
