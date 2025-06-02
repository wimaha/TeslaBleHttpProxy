package config

import (
	"os"
	"strconv"

	"github.com/charmbracelet/log"
)

var PublicKeyFile = "key/public.pem"
var PrivateKeyFile = "key/private.pem"

type Config struct {
	LogLevel          string
	HttpListenAddress string
	ScanTimeout       int // Seconds to scan for BLE devices
	CacheMaxAge       int // Seconds to cache BLE responses
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
		scanTimeout = "1" // default value
	}
	scanTimeoutInt, err := strconv.Atoi(scanTimeout)
	if err != nil {
		log.Error("Invalid scanTimeout value, using default (1)", "error", err)
		scanTimeoutInt = 1
	}
	log.Info("Env:", "scanTimeout", scanTimeoutInt)

	return &Config{
		LogLevel:          envLogLevel,
		HttpListenAddress: addr,
		CacheMaxAge:       cacheMaxAgeInt,
		ScanTimeout:       scanTimeoutInt,
	}
}

func InitConfig() {
	AppConfig = LoadConfig()
}
