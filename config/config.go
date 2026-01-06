package config

import (
	"os"
	"strconv"

	"github.com/charmbracelet/log"
)

var PublicKeyFile = "key/public.pem"
var PrivateKeyFile = "key/private.pem"

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
		log.SetLevel(log.DebugLevel)
		log.Debug("LogLevel set to debug")
	}
	if envLogLevel == "" {
		envLogLevel = "info"
	}

	addr := os.Getenv("httpListenAddress")
	if addr == "" {
		addr = ":8080"
	}
	log.Info("Env:", "httpListenAddress", addr)

	cacheMaxAge := os.Getenv("cacheMaxAge")
	if cacheMaxAge == "" {
		cacheMaxAge = "5" // default value
	}
	cacheMaxAgeInt, err := strconv.Atoi(cacheMaxAge)
	if err != nil {
		log.Error("Invalid cacheMaxAge value, using default (5)", "error", err)
		cacheMaxAgeInt = 5
	}
	log.Info("Env:", "cacheMaxAge", cacheMaxAgeInt)

	scanTimeout := os.Getenv("scanTimeout")
	if scanTimeout == "" {
		scanTimeout = "2" // default value
	}
	scanTimeoutInt, err := strconv.Atoi(scanTimeout)
	if err != nil {
		log.Error("Invalid scanTimeout value, using default (2)", "error", err)
		scanTimeoutInt = 2
	}
	log.Info("Env:", "scanTimeout", scanTimeoutInt)

	vehicleDataCacheTime := os.Getenv("vehicleDataCacheTime")
	if vehicleDataCacheTime == "" {
		vehicleDataCacheTime = "30" // default value: 30 seconds
	}
	vehicleDataCacheTimeInt, err := strconv.Atoi(vehicleDataCacheTime)
	if err != nil {
		log.Error("Invalid vehicleDataCacheTime value, using default (30)", "error", err)
		vehicleDataCacheTimeInt = 30
	}
	log.Info("Env:", "vehicleDataCacheTime", vehicleDataCacheTimeInt)

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
