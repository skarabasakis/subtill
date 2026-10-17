package http

import (
	"fmt"
	"net/http"
	"time"

	httpSwagger "github.com/swaggo/http-swagger/v2"
	swag "github.com/swaggo/swag/v2"

	_ "github.com/skarabasakis/subtill/internal/http/docs"
	"github.com/skarabasakis/subtill/internal/http/handlers"
	"github.com/skarabasakis/subtill/internal/http/middleware"
)

// ServerConfig configures the REST API server.
type ServerConfig struct {
	Addr  string
	Token string
}

// NewHandler returns the root HTTP handler for --api mode.
func NewHandler(token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handlers.Health)
	mux.HandleFunc("GET /swagger/doc.json", serveOpenAPIDoc)
	mux.Handle("GET /swagger/", httpSwagger.Handler(
		httpSwagger.InstanceName("swagger"),
		httpSwagger.URL("/swagger/doc.json"),
	))
	return middleware.BearerToken(token, "/swagger")(mux)
}

func serveOpenAPIDoc(w http.ResponseWriter, _ *http.Request) {
	doc, err := swag.ReadDoc("swagger")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(doc))
}

// ListenAndServe starts the REST API.
func ListenAndServe(cfg ServerConfig) error {
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           NewHandler(cfg.Token),
		ReadHeaderTimeout: 10 * time.Second,
	}
	fmt.Printf("subtill API listening on %s\n", cfg.Addr)
	return srv.ListenAndServe()
}
