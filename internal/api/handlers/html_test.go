package handlers

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
)

// Messages can carry user input (e.g. the VIN from send_key), so the dashboard must escape them.
func TestDashboardEscapesMessages(t *testing.T) {
	models.MainMessageStack.Push(models.Message{
		Title:   "Error",
		Message: `<script>alert(1)</script>`,
		Type:    models.Error,
	})

	rec := httptest.NewRecorder()
	ShowDashboard(os.DirFS("../../.."))(rec, httptest.NewRequest("GET", "/dashboard", nil))

	body := rec.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatalf("message rendered unescaped")
	}
	if !strings.Contains(body, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("escaped message not found in dashboard")
	}
}
