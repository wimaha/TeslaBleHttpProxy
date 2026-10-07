package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/wimaha/TeslaBleHttpProxy/internal/logging"
)

// Legacy key files for backward compatibility (will be removed in future)
var PublicKeyFile = "key/public.pem"
var PrivateKeyFile = "key/private.pem"

// GetActiveKeyFiles returns the private and public key file paths for the active key
func GetActiveKeyFiles() (string, string) {
	// Import here to avoid circular dependency
	activeRole := getActiveKeyRole()
	return getKeyFilesForRole(activeRole)
}

// getActiveKeyRole returns the active key role (internal function to avoid circular dependency)
func getActiveKeyRole() string {
	// Check for active key config file
	if data, err := os.ReadFile("key/active_key.json"); err == nil {
		var config struct {
			Role string `json:"role"`
		}
		if err := json.Unmarshal(data, &config); err == nil && config.Role != "" {
			return config.Role
		}
	}

	// Check for legacy keys (backward compatibility - should be migrated on startup)
	// Only return legacy if owner keys don't exist (migration might be pending)
	if _, err := os.Stat("key/private.pem"); err == nil {
		if _, err := os.Stat("key/owner/private.pem"); err != nil {
			return "" // Empty string indicates legacy keys (migration pending)
		}
		// Owner keys exist, so prefer them
		return "owner"
	}

	// Default to charging_manager if no keys exist (recommended for security)
	return "charging_manager"
}

// getKeyFilesForRole returns key file paths for a given role
// Uses filepath.Join for safe path construction
func getKeyFilesForRole(role string) (string, string) {
	// Support legacy single key format for backward compatibility
	if role == "" {
		// Check if legacy keys exist
		if _, err := os.Stat("key/private.pem"); err == nil {
			// If owner keys exist (migration happened), prefer them
			if _, err := os.Stat("key/owner/private.pem"); err == nil {
				// Owner keys exist, use them instead of legacy
				return "key/owner/private.pem", "key/owner/public.pem"
			}
			return "key/private.pem", "key/public.pem"
		}
		// Default to charging_manager if no legacy keys (recommended for security)
		role = "charging_manager"
	}

	// Validate role contains only safe characters (basic check)
	// Full validation should be done by ValidateRole in control package
	if strings.Contains(role, "..") || strings.Contains(role, "/") || strings.Contains(role, "\\") {
		// Path traversal attempt detected, default to charging_manager
		role = "charging_manager"
	}

	// New role-based key structure - use filepath.Join for safety
	keyDir := filepath.Join("key", role)
	return filepath.Join(keyDir, "private.pem"), filepath.Join(keyDir, "public.pem")
}

// GetKeyFilesForRole returns key file paths for a given role (public function)
func GetKeyFilesForRole(role string) (string, string) {
	return getKeyFilesForRole(role)
}

// Version is set at build time via linker flags
var Version = "*undefined*"

type Config struct {
	LogLevel             string
	HttpListenAddress    string
	ScanTimeout          int // Seconds to scan for BLE devices
	CacheMaxAge          int // Seconds for HTTP Cache-Control header max-age (used for body controller state responses). If set to 0, cache headers are disabled.
	VehicleDataCacheTime int // Seconds to cache VehicleData endpoint responses in memory. Each endpoint is cached separately per VIN.
}

var AppConfig *Config

func LoadConfig() *Config {
	envLogLevel := os.Getenv("logLevel")
	if envLogLevel == "debug" {
		logging.SetLevel(log.DebugLevel)
		logging.Debug("LogLevel set to debug")
	}
	if envLogLevel == "" {
		envLogLevel = "info"
	}

	addr := os.Getenv("httpListenAddress")
	if addr == "" {
		addr = ":8080"
	}
	logging.Info("Env:", "httpListenAddress", addr)

	cacheMaxAge := os.Getenv("cacheMaxAge")
	if cacheMaxAge == "" {
		cacheMaxAge = "5" // default value
	}
	cacheMaxAgeInt, err := strconv.Atoi(cacheMaxAge)
	if err != nil {
		logging.Error("Invalid cacheMaxAge value, using default (5)", "error", err)
		cacheMaxAgeInt = 5
	}
	logging.Info("Env:", "cacheMaxAge", cacheMaxAgeInt)

	scanTimeout := os.Getenv("scanTimeout")
	if scanTimeout == "" {
		scanTimeout = "2" // default value
	}
	scanTimeoutInt, err := strconv.Atoi(scanTimeout)
	if err != nil {
		logging.Error("Invalid scanTimeout value, using default (5)", "error", err)
		scanTimeoutInt = 5
	}
	logging.Info("Env:", "scanTimeout", scanTimeoutInt)

	vehicleDataCacheTime := os.Getenv("vehicleDataCacheTime")
	if vehicleDataCacheTime == "" {
		vehicleDataCacheTime = "30" // default value: 30 seconds
	}
	vehicleDataCacheTimeInt, err := strconv.Atoi(vehicleDataCacheTime)
	if err != nil {
		logging.Error("Invalid vehicleDataCacheTime value, using default (30)", "error", err)
		vehicleDataCacheTimeInt = 30
	}
	logging.Info("Env:", "vehicleDataCacheTime", vehicleDataCacheTimeInt)

	return &Config{
		LogLevel:             envLogLevel,
		HttpListenAddress:    addr,
		CacheMaxAge:          cacheMaxAgeInt,
		ScanTimeout:          scanTimeoutInt,
		VehicleDataCacheTime: vehicleDataCacheTimeInt,
	}
}

func InitConfig() {
	AppConfig = LoadConfig()
}
