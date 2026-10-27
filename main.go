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
	})
	// The built-in terminal pushes PTY output through window events.
	mainWindowRef = mainWindow

	if initialUrl != "" {
		mainWindow.OnWindowEvent(events.Common.WindowRuntimeReady, func(_ *application.WindowEvent) {
			mainWindow.EmitEvent("wgit:open-url", initialUrl)
		})
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
