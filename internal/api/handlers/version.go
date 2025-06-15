package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/wimaha/TeslaBleHttpProxy/config"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
)

func Version(w http.ResponseWriter, r *http.Request) {
	versionData := map[string]string{
		"version": config.Version,
	}

	versionJson, _ := json.Marshal(versionData)

	response := models.Ret{
		Response: models.Response{
			Result:   true,
			Reason:   "The request was successfully processed.",
			Response: versionJson,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
