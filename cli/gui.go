package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yashikota/enbu/app"
	"github.com/yashikota/enbu/auth"
	"github.com/yashikota/enbu/web"
)

const defaultPort = 3939

func newGUICommand(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "gui",
		Short: "Launch web-based management UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			frontend := web.FrontendFS()
			srv := web.NewServer(a, defaultClientID, frontend)

			addr := fmt.Sprintf("127.0.0.1:%d", defaultPort)
			url := fmt.Sprintf("http://%s", addr)

			fmt.Printf("Starting enbu UI at %s\n", url)
			fmt.Printf("CSRF Token: %s\n", srv.CSRFToken())

			_ = auth.OpenBrowser(url)

			return srv.ListenAndServe(addr)
		},
	}
}
