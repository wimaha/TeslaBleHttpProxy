package routes

import (
	"embed"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/handlers"
)

func SetupRoutes(static embed.FS, html embed.FS) *mux.Router {
	router := mux.NewRouter()

	// Define the endpoints
	///api/1/vehicles/{vehicle_tag}/command/set_charging_amps
	router.HandleFunc("/api/1/vehicles/{vin}/command/{command}", handlers.VehicleCommand).Methods("POST")
	router.HandleFunc("/api/1/vehicles/{vin}/{command}", handlers.VehicleEndpoint).Methods("GET", "POST")
	router.HandleFunc("/api/proxy/1/vehicles/{vin}/{command}", handlers.ProxyCommand).Methods("GET")
	router.HandleFunc("/dashboard", handlers.ShowDashboard(html)).Methods("GET")
	router.HandleFunc("/gen_keys", handlers.GenKeys).Methods("GET")
	router.HandleFunc("/remove_keys", handlers.RemoveKeys).Methods("GET")
	router.HandleFunc("/send_key", handlers.SendKey).Methods("POST")
	router.PathPrefix("/static/").Handler(http.FileServer(http.FS(static)))

	// Redirect / to /dashboard
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	})

	// 404 show route
	router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		http.Error(w, "404 page not found: "+path, http.StatusNotFound)
	})

	return router
}
