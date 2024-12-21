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

func main() {
	log.Info("TeslaBleHttpProxy 2.0.0 is loading ...")

	config := config.LoadConfig()

	control.SetupBleControl()

	router := routes.SetupRoutes(static)

	log.Info("TeslaBleHttpProxy is running!")
	log.Fatal(http.ListenAndServe(config.HttpListenAddress, router))
}
