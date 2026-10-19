module github.com/wimaha/TeslaBleHttpProxy

go 1.27.0

require (
	github.com/charmbracelet/log v1.0.0
	github.com/gorilla/mux v1.8.1
	github.com/teslamotors/vehicle-command v0.2.1
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/JuulLabs-OSS/cbgo v0.0.2 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/lipgloss v1.1.0 // indirect
	github.com/charmbracelet/x/ansi v0.11.9 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.15 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/cronokirby/saferith v0.33.0 // indirect
	github.com/go-ble/ble v0.0.0-20240122180141-8c5522f54333 // indirect
	github.com/go-logfmt/logfmt v0.6.1 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/mattn/go-colorable v0.1.16 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/mattn/go-runewidth v0.0.31 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/mgutz/logxi v0.0.0-20161027140823-aebf8a7d67ab // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/raff/goble v0.0.0-20200327175727-d63360dcfd80 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/sirupsen/logrus v1.10.2 // indirect
	github.com/xo/terminfo v1.2.0 // indirect
	golang.org/x/exp v0.0.0-20261014143641-f0251eb4aa3e // indirect
	golang.org/x/sys v0.49.0 // indirect
)

replace github.com/teslamotors/vehicle-command => github.com/wimaha/vehicle-command v0.0.8

replace github.com/go-ble/ble => github.com/wimaha/ble_BleConnectFix v0.0.0-20261018210806-b933489a83fc
