package desktop

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/skarabasakis/subtill/frontend"
	"github.com/skarabasakis/subtill/internal/desktop/services"
)

func newApp() *application.App {
	return application.New(application.Options{
		Name:        "subtill",
		Description: "Local subscription manager",
		Services: []application.Service{
			application.NewService(&services.AppService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(frontend.Dist),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Server: application.ServerOptions{
			Host: "127.0.0.1",
			Port: 9245,
		},
	})
}
