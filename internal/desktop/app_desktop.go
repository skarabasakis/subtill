//go:build !server

package desktop

import "github.com/wailsapp/wails/v3/pkg/application"

// Run starts the native desktop application.
func Run() error {
	app := newApp()
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "subtill",
		Width:            1000,
		Height:           618,
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})
	return app.Run()
}
