package services

// AppService is a bootstrap Wails service stub.
type AppService struct{}

// Ping returns a greeting for binding generation smoke tests.
func (a *AppService) Ping() string {
	return "pong"
}
