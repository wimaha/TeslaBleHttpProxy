package main

import (
	"embed"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/wimaha/TeslaBleHttpProxy/config"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/routes"
	"github.com/wimaha/TeslaBleHttpProxy/internal/ble/control"
)

//go:embed static/*
var static embed.FS

//go:embed html/*
var html embed.FS

func main() {
	log.Infof("TeslaBleHttpProxy %s is loading ...", config.Version)

	config.InitConfig()

	// Migrate legacy keys to owner role structure if they exist
	if err := control.MigrateLegacyKeys(); err != nil {
		log.Warn("Failed to migrate legacy keys", "error", err)
		// Continue anyway - migration failure shouldn't stop the application
	}

	control.SetupBleControl()

	router := routes.SetupRoutes(static, html)

	log.Info("TeslaBleHttpProxy is running!")
	log.Fatal(http.ListenAndServe(config.AppConfig.HttpListenAddress, router))
}
