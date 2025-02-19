package routes

import (
	"embed"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/gorilla/mux"
	"github.com/wimaha/TeslaBleHttpProxy/config"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/handlers"
)

func basePathHandler(from string, wrapped http.HandlerFunc) http.HandlerFunc {
	fromBase := config.AppConfig.ApiBaseUrl
	if from == "dashboard" {
		fromBase = config.AppConfig.DashboardBaseURL
	}
	return func(w http.ResponseWriter, r *http.Request) {
		basePath := fromBase
		if r.Header.Get("X-Ingress-Path") != "" {
			log.Debug("ingress path", "from", from, "path", r.Header.Get("X-Ingress-Path"))
			basePath = r.Header.Get("X-Ingress-Path")
		}
		if r.Header.Get("X-Forwarded-Prefix") != "" {
			log.Debug("forwarded prefix", "from", from, "path", r.Header.Get("X-Forwarded-Prefix"))
			basePath = r.Header.Get("X-Forwarded-Prefix")
		}
		r.SetPathValue("basePath", basePath)
		wrapped(w, r)
	}
}

func SetupRoutes(static embed.FS, html embed.FS) *mux.Router {
	router := mux.NewRouter()

	apiBase := config.AppConfig.ApiBaseUrl
	dashboardBase := config.AppConfig.DashboardBaseURL

	// Define the endpoints
	router.HandleFunc(apiBase+"/api/1/vehicles/{vin}/command/{command}", basePathHandler("api", handlers.VehicleCommand)).Methods("POST")
	router.HandleFunc(apiBase+"/api/1/vehicles/{vin}/{command}", basePathHandler("api", handlers.VehicleEndpoint)).Methods("GET", "POST")
	router.HandleFunc(apiBase+"/api/proxy/1/vehicles/{vin}/{command}", basePathHandler("api", handlers.ProxyCommand)).Methods("GET")
	router.HandleFunc(dashboardBase+"/dashboard", basePathHandler("dashboard", handlers.ShowDashboard(html))).Methods("GET")
	router.HandleFunc(dashboardBase+"/gen_keys", basePathHandler("dashboard", handlers.GenKeys)).Methods("GET")
	router.HandleFunc(dashboardBase+"/remove_keys", basePathHandler("dashboard", handlers.RemoveKeys)).Methods("GET")
	router.HandleFunc(dashboardBase+"/send_key", basePathHandler("dashboard", handlers.SendKey)).Methods("POST")

	// Static files
	staticHandler := http.FileServer(http.FS(static))
	if dashboardBase != "" {
		staticHandler = http.StripPrefix(dashboardBase, staticHandler)
	}
	router.PathPrefix(dashboardBase + "/static/").Handler(staticHandler)

	// Redirect / to /dashboard
	indexPath := dashboardBase
	if len(indexPath) == 0 || indexPath[len(indexPath)-1] != '/' {
		indexPath += "/"
	}
	router.HandleFunc(indexPath, basePathHandler("dashboard", func(w http.ResponseWriter, r *http.Request) {
		basePath := r.PathValue("basePath")
		log.Debugf("redirecting %s to %s", r.URL, basePath+"/dashboard")
		http.Redirect(w, r, basePath+"/dashboard", http.StatusSeeOther)
	}))

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
