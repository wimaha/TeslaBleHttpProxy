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
	router.HandleFunc("/api/1/vehicles/{vin}/command/{command}", handlers.Command).Methods("POST")
	router.HandleFunc("/api/1/vehicles/{vin}/vehicle_data", handlers.VehicleData).Methods("GET")
	router.HandleFunc("/api/1/vehicles/{vin}/body_controller_state", handlers.BodyControllerState).Methods("GET")
	router.HandleFunc("/api/proxy/1/version", handlers.Version).Methods("GET")
	router.HandleFunc("/dashboard", handlers.ShowDashboard(html)).Methods("GET")
	router.HandleFunc("/gen_keys", handlers.GenKeys).Methods("GET")
	router.HandleFunc("/remove_keys", handlers.RemoveKeys).Methods("GET")
	router.HandleFunc("/activate_key", handlers.ActivateKey).Methods("POST")
	router.HandleFunc("/send_key", handlers.SendKey).Methods("POST")
	router.PathPrefix("/static/").Handler(http.FileServer(http.FS(static)))

	return router
}
