//go:build server

package desktop

// Run starts Wails server mode (HTTP + bindings, no native window).
func Run() error {
	return newApp().Run()
}
