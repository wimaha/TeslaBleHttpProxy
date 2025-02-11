package main

import (
	"embed"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/teslamotors/vehicle-command/pkg/connector/ble"
	"github.com/wimaha/TeslaBleHttpProxy/config"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/routes"
	"github.com/wimaha/TeslaBleHttpProxy/internal/ble/control"
)

//go:embed static/*
var static embed.FS

//go:embed html/*
var html embed.FS

var Version = "*undefined*"

func main() {
	log.Infof("TeslaBleHttpProxy %s is loading ...", Version)

	config.InitConfig()
	level, _ := log.ParseLevel(config.AppConfig.LogLevel)
	log.SetLevel(level)

	btAdapterId := ""
	if config.AppConfig.BtAdapterID != "Default adapter" {
		btAdapterId = config.AppConfig.BtAdapterID
		log.Debug("Using Bluetooth adapter:", "adapter", btAdapterId)
	}
	err := ble.InitAdapterWithID(btAdapterId)
	if err != nil {
		log.Fatal("Failed to initialize Bluetooth adapter:", "err", err)
	}
	control.SetupBleControl()

	router := routes.SetupRoutes(static, html)

	log.Info("TeslaBleHttpProxy is running!")
	log.Fatal(http.ListenAndServe(config.AppConfig.HttpListenAddress, router))
}
