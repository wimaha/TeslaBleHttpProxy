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
	log.Info("TeslaBleHttpProxy", "httpListenAddress", addr)

	cacheMaxAge := os.Getenv("cacheMaxAge")
	if cacheMaxAge == "" {
		cacheMaxAge = "0" // default value
	}
	cacheMaxAgeInt, err := strconv.Atoi(cacheMaxAge)
	if err != nil {
		log.Error("Invalid cacheMaxAge value, using default (0)", "error", err)
		cacheMaxAgeInt = 0
	}

	return &Config{
		LogLevel:          envLogLevel,
		HttpListenAddress: addr,
		CacheMaxAge:       cacheMaxAgeInt,
	}
}

func InitConfig() {
	AppConfig = LoadConfig()
}
