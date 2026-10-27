package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// version and commit are the application version strings.
// They are set at build time via ldflags, e.g.:
//   -ldflags "-X main.version=v0.1.0 -X main.commit=d3f8249"
// or:
//   -ldflags "-X main.version=v0.1.0(d3f8249)"
var (
	version = "v0.1.0"
	commit  = "unknown"
)

func main() {
	app := NewApp(version, commit)

	err := wails.Run(&options.App{
		Title:  "twnoaa - NOAA Satellite Receiver & AI Analysis",
		Width:  1180,
		Height: 820,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 23, B: 42, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
