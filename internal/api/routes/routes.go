package routes

import (
	"io/fs"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/handlers"
)

// SetupRoutes returns the router wrapped in cross-origin protection: state-changing requests a
// browser sends from another site are rejected, while clients that send neither Origin nor
// Sec-Fetch-Site (evcc, curl, Home Assistant) pass unchanged.
func SetupRoutes(static fs.FS, html fs.FS) http.Handler {
	router := mux.NewRouter()

	// Define the endpoints
	///api/1/vehicles/{vehicle_tag}/command/set_charging_amps
	router.HandleFunc("/api/1/vehicles/{vin}/command/{command}", handlers.Command).Methods("POST")
	router.HandleFunc("/api/1/vehicles/{vin}/vehicle_data", handlers.VehicleData).Methods("GET")
	router.HandleFunc("/api/1/vehicles/{vin}/body_controller_state", handlers.BodyControllerState).Methods("GET")
	router.HandleFunc("/api/proxy/1/version", handlers.Version).Methods("GET")
	router.HandleFunc("/dashboard", handlers.ShowDashboard(html)).Methods("GET")
	router.HandleFunc("/logs", handlers.ShowLogViewer(html)).Methods("GET")
	router.HandleFunc("/api/logs", handlers.GetLogs).Methods("GET")
	router.HandleFunc("/api/logs/stats", handlers.GetLogStats).Methods("GET")
	router.HandleFunc("/gen_keys", handlers.GenKeys).Methods("POST")
	router.HandleFunc("/remove_keys", handlers.RemoveKeys).Methods("POST")
	router.HandleFunc("/activate_key", handlers.ActivateKey).Methods("POST")
	router.HandleFunc("/send_key", handlers.SendKey).Methods("POST")
	router.PathPrefix("/static/").Handler(http.FileServer(http.FS(static)))

	return http.NewCrossOriginProtection().Handler(router)
}
