package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	if err := wails.Run(&options.App{
		Title:            "VaultApp",
		Width:            1180,
		Height:           760,
		MinWidth:         860,
		MinHeight:        580,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 13, G: 18, B: 28, A: 1},
		OnStartup:        app.Startup,
		Bind:             []interface{}{app},
	}); err != nil {
		log.Fatal(err)
	}
}
