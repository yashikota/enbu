package main

import (
	"context"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/yashikota/enbu/app"
	"github.com/yashikota/enbu/assets"
	"github.com/yashikota/enbu/cli"
	"github.com/yashikota/enbu/desktop"
	"github.com/yashikota/enbu/web"
)

func main() {
	clientID := cli.DefaultClientID()
	if v := os.Getenv("ENBU_CLIENT_ID"); v != "" {
		clientID = v
	}

	core := desktop.NewService(app.New(), clientID)
	service := &DesktopService{service: core}
	core.SetDirectoryPicker(func(ctx context.Context) (string, error) {
		return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
			Title: "Select Git repository",
		})
	})
	core.SetBrowserOpener(func(url string) error {
		runtime.BrowserOpenURL(core.Context(), url)
		return nil
	})

	if err := wails.Run(&options.App{
		Title:  "enbu",
		Width:  1100,
		Height: 760,
		AssetServer: &assetserver.Options{
			Assets: web.FrontendFS(),
		},
		Mac: &mac.Options{
			About: &mac.AboutInfo{
				Title: "enbu",
				Icon:  assets.Icon,
			},
		},
		Linux: &linux.Options{
			Icon:        assets.Icon,
			ProgramName: "enbu",
		},
		OnStartup: core.Startup,
		Bind: []interface{}{
			service,
		},
	}); err != nil {
		log.Fatal(err)
	}
}
