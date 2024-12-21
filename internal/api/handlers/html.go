package handlers

import (
	"embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/template"

	"github.com/charmbracelet/log"
	"github.com/wimaha/TeslaBleHttpProxy/config"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
	"github.com/wimaha/TeslaBleHttpProxy/internal/ble/control"
)

type DashboardParams struct {
	PrivateKey    string
	PublicKey     string
	ShouldGenKeys bool
	Messages      []models.Message
}

func ShowDashboard(w http.ResponseWriter, r *http.Request) {
	var shouldGenKeys = true
	var privateKey = "- missing -"
	if _, err := os.Stat(config.PrivateKeyFile); err == nil {
		privateKey = "private.pem"
		shouldGenKeys = false
	}

	var publicKey = "- missing -"
	if _, err := os.Stat(config.PublicKeyFile); err == nil {
		publicKey = "public.pem"
		shouldGenKeys = false
	}

	messages := models.MainMessageStack.PopAll()

	p := DashboardParams{
		PrivateKey:    privateKey,
		PublicKey:     publicKey,
		ShouldGenKeys: shouldGenKeys,
		Messages:      messages,
	}
	if err := Dashboard(w, p, ""); err != nil {
		log.Error("error showing dashboard", "error", err)
	}
}

func GenKeys(w http.ResponseWriter, r *http.Request) {
	err := control.CreatePrivateAndPublicKeyFile()

	if err == nil {
		control.SetupBleControl()
		models.MainMessageStack.Push(models.Message{
			Title:   "Success",
			Message: "Keys successfully generated and saved.",
			Type:    models.Success,
		})
	} else {
		models.MainMessageStack.Push(models.Message{
			Title:   "Error",
			Message: err.Error(),
			Type:    models.Error,
		})
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func RemoveKeys(w http.ResponseWriter, r *http.Request) {
	err1, err2 := control.RemoveKeyFiles()

	if err1 != nil {
		models.MainMessageStack.Push(models.Message{
			Title:   "Error",
			Message: err1.Error(),
			Type:    models.Error,
		})
	}
	if err2 != nil {
		models.MainMessageStack.Push(models.Message{
			Title:   "Error",
			Message: err2.Error(),
			Type:    models.Error,
		})
	}
	if err1 == nil && err2 == nil {
		models.MainMessageStack.Push(models.Message{
			Title:   "Success",
			Message: "Keys successfully removed.",
			Type:    models.Success,
		})
	}

	control.CloseBleControl()
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func SendKey(w http.ResponseWriter, r *http.Request) {
	defer func() {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}()

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		vin := r.FormValue("VIN")

		if vin == "" {
			models.MainMessageStack.Push(models.Message{
				Title:   "Error",
				Message: "No VIN entered.",
				Type:    models.Error,
			})
			return
		}

		err := control.SendKeysToVehicle(vin)

		if err != nil {
			models.MainMessageStack.Push(models.Message{
				Title:   "Error",
				Message: err.Error(),
				Type:    models.Error,
			})
		} else {
			models.MainMessageStack.Push(models.Message{
				Title:   "Success",
				Message: fmt.Sprintf("Sent add-key request to %s. Confirm by tapping NFC card on center console.", vin),
				Type:    models.Success,
			})
		}
	}
}

//go:embed *
var html embed.FS

func parse(file string) *template.Template {
	return template.Must(
		template.New("layout.html").ParseFS(html, "layout.html", file))
}

func Dashboard(w io.Writer, p DashboardParams, partial string) error {
	if partial == "" {
		partial = "layout.html"
	}
	return dashboard.ExecuteTemplate(w, partial, p)
}

var dashboard = parse("dashboard.html")
