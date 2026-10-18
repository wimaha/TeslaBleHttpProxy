package routes

import (
	"io/fs"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/handlers"
	"github.com/wimaha/TeslaBleHttpProxy/internal/logging"
)

// SetupRoutes returns the router wrapped in cross-origin protection: state-changing requests a
// browser sends from another site are rejected, while clients that send neither Origin nor
// Sec-Fetch-Site (evcc, curl, Home Assistant) pass unchanged. trustedOrigins are exempt, for a
// reverse proxy that rewrites the Host header on plain HTTP, where browsers send no Sec-Fetch-Site.
func SetupRoutes(static fs.FS, html fs.FS, trustedOrigins []string) http.Handler {
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

	protection := http.NewCrossOriginProtection()
	for _, origin := range trustedOrigins {
		if err := protection.AddTrustedOrigin(normalizeOrigin(origin)); err != nil {
			logging.Error("Ignoring invalid trusted origin", "origin", origin, "error", err)
		}
	}
	return protection.Handler(router)
}

// normalizeOrigin turns a configured origin into the form browsers send in the Origin header,
// which is compared verbatim: lowercase, no trailing slash, no default port. Anything it cannot
// parse is returned unchanged so AddTrustedOrigin reports it.
func normalizeOrigin(origin string) string {
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(origin), "/"))
	if err != nil || u.Host == "" {
		return origin
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	if strings.Contains(host, ":") {
		host = "[" + host + "]" // IPv6 literal
	}
	if port := u.Port(); port != "" && !(scheme == "http" && port == "80") && !(scheme == "https" && port == "443") {
		host += ":" + port
	}
	u.Scheme, u.Host = scheme, host
	return u.String()
}
