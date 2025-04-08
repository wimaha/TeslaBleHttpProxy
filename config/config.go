package config

import (
	"fmt"
	"net/url"
	"os"

	"github.com/akamensky/argparse"
	"github.com/charmbracelet/log"
)

var PublicKeyFile = "key/public.pem"
var PrivateKeyFile = "key/private.pem"

type Config struct {
	LogLevel          string
	HttpListenAddress string
	ScanTimeout       int    // Seconds to scan for BLE beacons during device scan (0 = max)
	CacheMaxAge       int    // Seconds to cache BLE responses
	DashboardBaseURL  string // Base URL for proxying dashboard (Useful if the proxy is behind a reverse proxy)
	ApiBaseUrl        string // Base URL for proxying BLE commands (Useful if the proxy is behind a reverse proxy)
}

var AppConfig *Config

func LoadConfig() *Config {
	parser := argparse.NewParser("TeslaBleHttpProxy", "Proxy for Tesla BLE commands over HTTP")
	logLevel := parser.String("l", "logLevel", &argparse.Options{Help: "Log level (DEBUG, INFO, WARN, ERROR, FATAL)", Default: "INFO", Validate: func(args []string) error {
		if _, err := log.ParseLevel(args[0]); err != nil {
			return err
		}
		return nil
	}})
	httpListenAddress := parser.String("b", "httpListenAddress", &argparse.Options{Help: "HTTP bind address", Default: ":8080", Validate: func(args []string) error {
		// Check if the proxy host is a valid URL
		url, err := url.Parse(fmt.Sprintf("//%s", args[0]))
		if err != nil {
			return fmt.Errorf("invalid bind address (%s)", err)
		}
		if url.Path != "" {
			return fmt.Errorf("bind address must not contain a path or scheme")
		}
		return nil
	}})
	scanTimeout := parser.Int("s", "scanTimeout", &argparse.Options{Help: "Time in seconds to scan for BLE beacons during device scan (0 = max)", Default: 1})
	cacheMaxAge := parser.Int("c", "cacheMaxAge", &argparse.Options{Help: "Time in seconds for Cache-Control header (0 = no cache)", Default: 5})
	keys := parser.String("k", "keys", &argparse.Options{Help: "Path to public and private keys", Default: "key", Validate: func(args []string) error {
		f, err := os.Stat(args[0])
		if err != nil {
			return fmt.Errorf("failed to find keys directory (%s)", err)
		}
		if !f.IsDir() {
			return fmt.Errorf("keys is not a directory")
		}
		return nil
	}})
	dashboardBaseUrl := parser.String("d", "dashboardBaseUrl", &argparse.Options{Help: "Base URL for dashboard (Useful if the proxy is behind a reverse proxy)", Default: ""})
	apiBaseUrl := parser.String("a", "apiBaseUrl", &argparse.Options{Help: "Base URL for proxying API commands", Default: ""})
	// Inject environment variables as command line arguments
	args := os.Args
	for _, arg := range parser.GetArgs() {
		if arg.GetPositional() || arg.GetLname() == "help" {
			continue
		}
		osArg := os.Getenv(arg.GetLname())
		if osArg != "" {
			args = append(args, fmt.Sprintf("--%s=%s", arg.GetLname(), osArg))
		}
	}

	err := parser.Parse(args)
	if err != nil {
		log.Fatal("Failed to parse arguments", "error", err)
	}

	PublicKeyFile = fmt.Sprintf("%s/public.pem", *keys)
	PrivateKeyFile = fmt.Sprintf("%s/private.pem", *keys)

	return &Config{
		LogLevel:          *logLevel,
		HttpListenAddress: *httpListenAddress,
		ScanTimeout:       *scanTimeout,
		CacheMaxAge:       *cacheMaxAge,
		DashboardBaseURL:  *dashboardBaseUrl,
		ApiBaseUrl:        *apiBaseUrl,
	}
}

func InitConfig() {
	AppConfig = LoadConfig()
}
