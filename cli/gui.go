package cli

import (
	"fmt"
	"os"

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
			clientID := defaultClientID
			if v := os.Getenv("ENBU_CLIENT_ID"); v != "" {
				clientID = v
			}
			if v := os.Getenv("ENBU_CLIENT_SECRET"); v != "" {
				web.ClientSecret = v
			}

			frontend := web.FrontendFS()
			srv := web.NewServer(a, clientID, frontend)

			addr := fmt.Sprintf("127.0.0.1:%d", defaultPort)
			url := fmt.Sprintf("http://%s", addr)

			fmt.Printf("Starting enbu GUI at %s\n", url)

			go func() {
				_ = auth.OpenBrowser(url)
			}()

			return srv.ListenAndServe(addr)
		},
	}
}
