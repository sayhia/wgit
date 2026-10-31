package main

import (
	"embed"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
//
//go:embed all:frontend/dist
var assets embed.FS

//go:embed resources/icon.png
var appIcon []byte

func main() {
	var initialUrl string
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "wgit://") {
			initialUrl = arg
			SetInitialDeepLink(initialUrl)
			break
		}
	}

	app := application.New(application.Options{
		Name:        "wgit",
		Description: "wgit - Modern Universal Git Client",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(&GitService{}),
			application.NewService(&AiService{}),
			application.NewService(&AssetService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/__log" {
						log.Printf("[WKWebView Client Log] %s", r.URL.Query().Get("msg"))
						w.WriteHeader(http.StatusOK)
						return
					}
					next.ServeHTTP(w, r)
				})
			},
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.wgit.app",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				if mainWindowRef != nil {
					mainWindowRef.Focus()
					for _, arg := range data.Args {
						if strings.HasPrefix(arg, "wgit://") {
							mainWindowRef.EmitEvent("wgit:open-url", arg)
							return
						}
					}
				}
			},
		},
	})

	app.Event.OnApplicationEvent(events.Common.ApplicationLaunchedWithUrl, func(e *application.ApplicationEvent) {
		if e != nil && e.Context() != nil {
			rawURL := e.Context().URL()
			if rawURL != "" {
				SetInitialDeepLink(rawURL)
				if mainWindowRef != nil {
					mainWindowRef.Focus()
					mainWindowRef.EmitEvent("wgit:open-url", rawURL)
				}
			}
		}
	})

	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "wgit",
		Width:     1380,
		Height:    880,
		MinWidth:  960,
		MinHeight: 640,
		Mac: application.MacWindow{
			TitleBar: application.MacTitleBarHiddenInset,
			Backdrop: application.MacBackdropTranslucent,
		},
		BackgroundColour: application.NewRGB(13, 17, 23),
		URL:              "/",
		DevToolsEnabled:  true,
		// WKWebView never exposes `File.path`, so an HTML5 drop cannot tell us
		// which folder was dropped. With this on, Wails resolves the element
		// carrying `data-file-drop-target`, hands the absolute paths to the Go
		// side, and we forward them to the UI.
		EnableFileDrop: true,
	})
	// The built-in terminal pushes PTY output through window events.
	mainWindowRef = mainWindow

	// Folder dropped on the window → open it as a repository.
	mainWindow.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		if event == nil || event.Context() == nil {
			return
		}
		files := event.Context().DroppedFiles()
		if len(files) == 0 {
			return
		}
		// The frontend validates that the path is a repository and reports the
		// failure there, so a wrong drop gets a message instead of silence.
		mainWindow.EmitEvent("wgit:open-repo-path", files[0])
	})

	// System Tray Integration
	tray := app.SystemTray.New()
	tray.SetIcon(appIcon)
	tray.SetTooltip("wgit - Modern Universal Git Client")
	trayMenu := app.NewMenu()
	trayMenu.Add("显示主窗口 (Show Window)").OnClick(func(_ *application.Context) {
		if mainWindowRef != nil {
			mainWindowRef.Show()
			mainWindowRef.Focus()
		}
	})
	trayMenu.Add("隐藏窗口 (Hide Window)").OnClick(func(_ *application.Context) {
		if mainWindowRef != nil {
			mainWindowRef.Hide()
		}
	})
	trayMenu.AddSeparator()
	trayMenu.Add("退出 (Quit)").OnClick(func(_ *application.Context) {
		app.Quit()
	})
	tray.SetMenu(trayMenu)
	tray.OnClick(func() {
		if mainWindowRef != nil {
			if mainWindowRef.IsVisible() {
				mainWindowRef.Hide()
			} else {
				mainWindowRef.Show()
				mainWindowRef.Focus()
			}
		}
	})

	if initialUrl != "" {
		mainWindow.OnWindowEvent(events.Common.WindowRuntimeReady, func(_ *application.WindowEvent) {
			mainWindow.EmitEvent("wgit:open-url", initialUrl)
		})
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
