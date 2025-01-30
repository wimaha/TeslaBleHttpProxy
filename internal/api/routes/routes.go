package routes

import (
	"embed"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/gorilla/mux"
	"github.com/wimaha/TeslaBleHttpProxy/config"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/handlers"
)

func SetupRoutes(static embed.FS, html embed.FS) *mux.Router {
	router := mux.NewRouter()

	base := config.AppConfig.ProxyBaseURL

	// Define the endpoints
	router.HandleFunc(base+"/api/1/vehicles/{vin}/command/{command}", handlers.VehicleCommand).Methods("POST")
	router.HandleFunc(base+"/api/1/vehicles/{vin}/{command}", handlers.VehicleEndpoint).Methods("GET", "POST")
	router.HandleFunc(base+"/api/proxy/1/vehicles/{vin}/{command}", handlers.ProxyCommand).Methods("GET")
	router.HandleFunc(base+"/dashboard", handlers.ShowDashboard(html)).Methods("GET")
	router.HandleFunc(base+"/gen_keys", handlers.GenKeys).Methods("GET")
	router.HandleFunc(base+"/remove_keys", handlers.RemoveKeys).Methods("GET")
	router.HandleFunc(base+"/send_key", handlers.SendKey).Methods("POST")

	// Static files
	staticHandler := http.FileServer(http.FS(static))
	if base != "" {
		staticHandler = http.StripPrefix(base, staticHandler)
	}
	router.PathPrefix(base + "/static/").Handler(staticHandler)

	// Redirect / to /dashboard
	indexPath := base
	if len(indexPath) == 0 {
		indexPath = "/"
	}
	router.HandleFunc(indexPath, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, base+"/dashboard", http.StatusSeeOther)
	})

	router.MethodNotAllowedHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debug("405 method not allowed", "method", r.Method, "path", r.URL.Path)
		method := r.Method
		path := r.URL.Path
		http.Error(w, "405 method not allowed: "+method+" "+path, http.StatusMethodNotAllowed)
	})

	// 404 show route
	router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debug("404 page not found", "path", r.URL.Path)
		path := r.URL.Path
		http.Error(w, "404 page not found: "+path, http.StatusNotFound)
	})

	return router
}
