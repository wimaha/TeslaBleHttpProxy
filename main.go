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
	log.Info("TeslaBleHttpProxy 2.0.0 is loading ...")

	config := config.LoadConfig()

	control.SetupBleControl()

	router := routes.SetupRoutes(static, html)

	log.Info("TeslaBleHttpProxy is running!")
	log.Fatal(http.ListenAndServe(config.HttpListenAddress, router))
}
