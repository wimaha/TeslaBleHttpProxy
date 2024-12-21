package config

import (
	"os"

	"github.com/charmbracelet/log"
)

var PublicKeyFile = "key/public.pem"
var PrivateKeyFile = "key/private.pem"

type Config struct {
	LogLevel          string
	HttpListenAddress string
}

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

	return &Config{
		LogLevel:          envLogLevel,
		HttpListenAddress: addr,
	}
}
