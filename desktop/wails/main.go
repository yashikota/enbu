package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

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

func setupLogger() *os.File {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	dir = filepath.Join(dir, "enbu")
	_ = os.MkdirAll(dir, 0o700)
	name := filepath.Join(dir, fmt.Sprintf("enbu-%s.log", time.Now().Format("20060102-150405")))
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil
	}
	h := slog.NewTextHandler(f, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	})
	slog.SetDefault(slog.New(h))
	slog.Info("enbu desktop starting", "log", name)
	return f
}

func main() {
	logFile := setupLogger()
	if logFile != nil {
		defer logFile.Close()
	}

	clientID := cli.DefaultClientID()
	if v := os.Getenv("ENBU_CLIENT_ID"); v != "" {
		clientID = v
	}
	slog.Debug("config", "clientID", clientID)

	core := desktop.NewService(app.New(), clientID)
	service := &DesktopService{service: core}
	core.SetDirectoryPicker(func(ctx context.Context) (string, error) {
		slog.Debug("OpenDirectoryDialog called")
		return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
			Title: "Select Git repository",
		})
	})
	core.SetBrowserOpener(func(url string) error {
		slog.Info("BrowserOpenURL", "url", url)
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
		slog.Error("wails.Run failed", "err", err)
		os.Exit(1)
	}
}
