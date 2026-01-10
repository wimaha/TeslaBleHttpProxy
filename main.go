package main

import (
	"embed"
	"net/http"

	"github.com/wimaha/TeslaBleHttpProxy/config"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/routes"
	"github.com/wimaha/TeslaBleHttpProxy/internal/ble/control"
	"github.com/wimaha/TeslaBleHttpProxy/internal/logging"
)

//go:embed static/*
var static embed.FS

//go:embed html/*
var html embed.FS

func main() {
	// Initialize log handler to capture all logs
	logging.InitLogHandler()

	logging.Infof("TeslaBleHttpProxy %s is loading ...", config.Version)

	config.InitConfig()

	// Migrate legacy keys to owner role structure if they exist
	if err := control.MigrateLegacyKeys(); err != nil {
		logging.Warn("Failed to migrate legacy keys", "error", err)
		// Continue anyway - migration failure shouldn't stop the application
	}

	control.SetupBleControl()

	router := routes.SetupRoutes(static, html)

	logging.Info("TeslaBleHttpProxy is running!")
	if err := http.ListenAndServe(config.AppConfig.HttpListenAddress, router); err != nil {
		logging.Fatal("Server failed", "error", err)
	}
}
