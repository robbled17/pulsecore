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
	defer func() {
		if r := recover(); r != nil {
			logCrash("main", r)
		}
	}()
	_ = appIcon // keep embed referenced; Wails packages build/windows/icon.ico into the EXE
	for _, arg := range os.Args[1:] {
		if strings.EqualFold(arg, "--dev") {
			appDevMode = true
		}
	}
	app := NewApp()

	isWin11 := isWindows11OrGreater()
	var bgColour *options.RGBA
	var winOptions *windows.Options

	if isWin11 {
		bgColour = &options.RGBA{R: 0, G: 0, B: 0, A: 0}
		winOptions = &windows.Options{
			WebviewIsTransparent:              true,
			WindowIsTranslucent:               true,
			DisableWindowIcon:                 false,
			DisableFramelessWindowDecorations: false,
			BackdropType:                      windows.Mica,
			Theme:                             windows.SystemDefault,
		}
	} else {
		// Windows 10 or earlier (< build 22000):
		// Disable Acrylic/Mica/Translucent backdrop to eliminate flickering transparent trail next to scrollbar
		bgColour = &options.RGBA{R: 11, G: 15, B: 25, A: 255}
		winOptions = &windows.Options{
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			DisableWindowIcon:                 false,
			DisableFramelessWindowDecorations: false,
			BackdropType:                      windows.None,
			Theme:                             windows.SystemDefault,
		}
	}

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
		BackgroundColour:  bgColour,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:     app.startup,
		OnShutdown:    app.shutdown,
		OnBeforeClose: app.beforeClose,
		Bind: []interface{}{
			app,
		},
		Windows: winOptions,
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
