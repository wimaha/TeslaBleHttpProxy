package html

import (
	"embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/template"

	"github.com/charmbracelet/log"
	"github.com/wimaha/TeslaBleHttpProxy/control"
)

type DashboardParams struct {
	PrivateKey    string
	PublicKey     string
	ShouldGenKeys bool
	Messages      []Message
}

func ShowDashboard(w http.ResponseWriter, r *http.Request) {
	var shouldGenKeys = true
	var privateKey = "- missing -"
	if _, err := os.Stat(control.PrivateKeyFile); err == nil {
		privateKey = "private.pem"
		shouldGenKeys = false
	}

	var publicKey = "- missing -"
	if _, err := os.Stat(control.PublicKeyFile); err == nil {
		publicKey = "public.pem"
		shouldGenKeys = false
	}

	messages := MainMessageStack.PopAll()

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
		MainMessageStack.Push(Message{
			Title:   "Success",
			Message: "Keys successfully generated and saved.",
			Type:    Success,
		})
	} else {
		MainMessageStack.Push(Message{
			Title:   "Error",
			Message: err.Error(),
			Type:    Error,
		})
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func RemoveKeys(w http.ResponseWriter, r *http.Request) {
	err1, err2 := control.RemoveKeyFiles()

	if err1 != nil {
		MainMessageStack.Push(Message{
			Title:   "Error",
			Message: err1.Error(),
			Type:    Error,
		})
	}
	if err2 != nil {
		MainMessageStack.Push(Message{
			Title:   "Error",
			Message: err2.Error(),
			Type:    Error,
		})
	}
	if err1 == nil && err2 == nil {
		MainMessageStack.Push(Message{
			Title:   "Success",
			Message: "Keys successfully removed.",
			Type:    Success,
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
			MainMessageStack.Push(Message{
				Title:   "Error",
				Message: "No VIN entered.",
				Type:    Error,
			})
			return
		}

		err := control.SendKeysToVehicle(vin)

		if err != nil {
			MainMessageStack.Push(Message{
				Title:   "Error",
				Message: err.Error(),
				Type:    Error,
			})
		} else {
			MainMessageStack.Push(Message{
				Title:   "Success",
				Message: fmt.Sprintf("Sent add-key request to %s. Confirm by tapping NFC card on center console.", vin),
				Type:    Success,
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
