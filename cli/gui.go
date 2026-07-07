package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/yashikota/enbu/app"
	"github.com/yashikota/enbu/auth"
	"github.com/yashikota/enbu/web"
)

const defaultPort = 3939

func newGUICommand(a *app.App) *cobra.Command {
	var port int
	var noBrowser bool

	cmd := &cobra.Command{
		Use:   "gui",
		Short: "Launch web-based management UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGUI(cmd.Context(), a, port, noBrowser)
		},
	}

	cmd.Flags().IntVar(&port, "port", defaultPort, "port for the local GUI server")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "do not open the browser automatically")
	return cmd
}

func runGUI(ctx context.Context, a *app.App, port int, noBrowser bool) error {
	clientID := defaultClientID
	if v := os.Getenv("ENBU_CLIENT_ID"); v != "" {
		clientID = v
	}
	if v := os.Getenv("ENBU_CLIENT_SECRET"); v != "" {
		web.ClientSecret = v
	}

	frontend := web.FrontendFS()
	srv := web.NewServer(a, clientID, frontend)

	ln, err := listenGUI(port)
	if err != nil {
		return err
	}
	defer func() { _ = ln.Close() }()

	httpSrv := &http.Server{Handler: srv}
	go func() {
		<-ctx.Done()
		_ = httpSrv.Close()
	}()

	url := fmt.Sprintf("http://%s", ln.Addr().String())
	fmt.Printf("Starting enbu GUI at %s\n", url)

	if !noBrowser {
		go func() {
			_ = auth.OpenBrowser(url)
		}()
	}

	if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func listenGUI(port int) (net.Listener, error) {
	if port == 0 {
		return net.Listen("tcp", "127.0.0.1:0")
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err == nil {
		return ln, nil
	}

	if port != defaultPort {
		return nil, err
	}

	for p := defaultPort + 1; p < defaultPort+20; p++ {
		ln, listenErr := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if listenErr == nil {
			return ln, nil
		}
	}
	return nil, err
}
