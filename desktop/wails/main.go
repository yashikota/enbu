package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/enbu-net/enbu/app"
	"github.com/enbu-net/enbu/assets"
	"github.com/enbu-net/enbu/desktop"
	"github.com/enbu-net/enbu/web"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type slogWailsLogger struct{}

func (slogWailsLogger) Print(message string) { slog.Debug("wails print", "message", message) }
func (slogWailsLogger) Trace(message string) { slog.Debug("wails trace", "message", message) }
func (slogWailsLogger) Debug(message string) { slog.Debug("wails debug", "message", message) }
func (slogWailsLogger) Info(message string)  { slog.Info("wails info", "message", message) }
func (slogWailsLogger) Warning(message string) {
	slog.Warn("wails warning", "message", message)
}
func (slogWailsLogger) Error(message string) { slog.Error("wails error", "message", message) }
func (slogWailsLogger) Fatal(message string) { slog.Error("wails fatal", "message", message) }

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

func showWindow(ctx context.Context) {
	if ctx == nil {
		return
	}
	runtime.WindowShow(ctx)
	runtime.WindowUnminimise(ctx)
	runtime.WindowCenter(ctx)
}

func main() {
	logFile := setupLogger()
	if logFile != nil {
		defer func() {
			_ = logFile.Close()
		}()
	}

	core := desktop.NewService(app.New())
	service := &DesktopService{service: core}
	core.SetDirectoryPicker(func(ctx context.Context) (string, error) {
		slog.Debug("OpenDirectoryDialog called")
		return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
			Title: "Select Git repository",
		})
	})
	core.SetBrowserOpener(func(url string) error {
		slog.Info("BrowserOpenURL")
		runtime.BrowserOpenURL(core.Context(), url)
		return nil
	})

	if err := wails.Run(&options.App{
		Title:              "enbu",
		Width:              1100,
		Height:             760,
		Logger:             slogWailsLogger{},
		LogLevel:           logger.TRACE,
		LogLevelProduction: logger.TRACE,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "net.enbu.desktop",
			OnSecondInstanceLaunch: func(data options.SecondInstanceData) {
				slog.Info("second instance launch", "args", data.Args, "cwd", data.WorkingDirectory)
				showWindow(core.Context())
			},
		},
		AssetServer: &assetserver.Options{
			Assets: web.FrontendFS(),
		},
		Mac: &mac.Options{
			About: &mac.AboutInfo{
				Title: "enbu",
				Icon:  assets.Icon,
			},
			OnUrlOpen: func(_ string) {
				slog.Info("URL opened")
				showWindow(core.Context())
			},
		},
		Linux: &linux.Options{
			Icon:        assets.Icon,
			ProgramName: "enbu",
		},
		OnStartup: core.Startup,
		OnDomReady: func(ctx context.Context) {
			slog.Info("Wails.OnDomReady called")
		},
		Bind: []interface{}{
			service,
		},
	}); err != nil {
		slog.Error("wails.Run failed", "err", err)
		os.Exit(1)
	}
}
