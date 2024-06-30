package html

import (
	"embed"
	"io"
	"net/http"
	"text/template"
)

type DashboardParams struct {
	PrivateKey string
	PublicKey  string
}

func ShowDashboard(w http.ResponseWriter, r *http.Request) {
	p := DashboardParams{
		PrivateKey: "test1",
		PublicKey:  "test2",
	}
	Dashboard(w, p, "")
}

//go:embed *
var files embed.FS

func parse(file string) *template.Template {
	return template.Must(
		template.New("layout.html").ParseFS(files, "layout.html", file))
}

func Dashboard(w io.Writer, p DashboardParams, partial string) error {
	if partial == "" {
		partial = "layout.html"
	}
	return dashboard.ExecuteTemplate(w, partial, p)
}

var dashboard = parse("dashboard.html")
