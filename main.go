package main

import (
	"embed"
	"log"
	"net/http"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name:        "wgit",
		Description: "wgit - Modern Universal Git Client",
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

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
