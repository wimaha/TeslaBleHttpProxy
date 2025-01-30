package handlers

import (
	"fmt"
	"io"
	"io/fs"
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
	BaseUrl       string
}

func ShowDashboard(html fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
			BaseUrl:       config.AppConfig.ProxyBaseURL,
		}
		if err := Dashboard(w, p, "", html); err != nil {
			log.Error("error showing dashboard", "error", err)
		}
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
	base := config.AppConfig.ProxyBaseURL
	http.Redirect(w, r, base+"/dashboard", http.StatusSeeOther)
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
	base := config.AppConfig.ProxyBaseURL
	http.Redirect(w, r, base+"/dashboard", http.StatusSeeOther)
}

func SendKey(w http.ResponseWriter, r *http.Request) {
	defer func() {
		base := config.AppConfig.ProxyBaseURL
		http.Redirect(w, r, base+"/dashboard", http.StatusSeeOther)
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

func parse(file string, html fs.FS) *template.Template {
	return template.Must(
		template.New("html/layout.html").ParseFS(html, "html/layout.html", "html/"+file))
}

func Dashboard(w io.Writer, p DashboardParams, partial string, html fs.FS) error {
	if partial == "" {
		partial = "layout.html"
	}
	return parse("dashboard.html", html).ExecuteTemplate(w, partial, p)
}
