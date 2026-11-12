package main

import (
	"embed"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// App icon source for tray / docs; Windows EXE icon comes from build/windows/icon.ico.
//
//go:embed build/appicon.png
var appIcon []byte

func main() {
	_ = appIcon // keep embed referenced; Wails packages build/windows/icon.ico into the EXE
	for _, arg := range os.Args[1:] {
		if strings.EqualFold(arg, "--dev") {
			appDevMode = true
		}
	}
	app := NewApp()

	err := wails.Run(&options.App{
		Title:             "PulseCore",
		Width:             1280,
		Height:            800,
		MinWidth:          760,
		MinHeight:         560,
		DisableResize:     false,
		Fullscreen:        false,
		Frameless:         true,
		StartHidden:       false,
		HideWindowOnClose: false,
		BackgroundColour:  &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:     app.startup,
		OnShutdown:    app.shutdown,
		OnBeforeClose: app.beforeClose,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              true,
			WindowIsTranslucent:               true,
			DisableWindowIcon:                 false,
			DisableFramelessWindowDecorations: false,
			BackdropType:                      windows.Mica,
			Theme:                             windows.SystemDefault,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
