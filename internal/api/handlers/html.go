package handlers

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"text/template"

	"github.com/charmbracelet/log"
	"github.com/wimaha/TeslaBleHttpProxy/config"
	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
	"github.com/wimaha/TeslaBleHttpProxy/internal/ble/control"
)

type KeyInfo struct {
	Role        string
	DisplayName string
	IsActive    bool
	Exists      bool
}

type DashboardParams struct {
	Keys          []KeyInfo
	ActiveKeyRole string
	ShouldGenKeys bool
	Messages      []models.Message
	Version       string
}

func ShowDashboard(html fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get all available keys
		availableRoles := control.ListAvailableKeys()
		activeRole := control.GetActiveKeyRole()

		// Build key info list (exclude legacy - it's automatically migrated)
		allRoles := []string{control.KeyRoleOwner, control.KeyRoleChargingManager}
		keys := make([]KeyInfo, 0)
		
		for _, role := range allRoles {
			exists := control.KeyExists(role)
			keys = append(keys, KeyInfo{
				Role:        role,
				DisplayName: control.GetKeyRoleDisplayName(role),
				IsActive:    role == activeRole,
				Exists:      exists,
			})
		}

		shouldGenKeys := len(availableRoles) == 0
		messages := models.MainMessageStack.PopAll()

		p := DashboardParams{
			Keys:          keys,
			ActiveKeyRole: activeRole,
			ShouldGenKeys: shouldGenKeys,
			Messages:      messages,
			Version:       config.Version,
		}
		if err := Dashboard(w, p, "", html); err != nil {
			log.Error("Error showing dashboard", "Error", err)
		}
	}
}

func GenKeys(w http.ResponseWriter, r *http.Request) {
	// Get role from query parameter
	role := r.URL.Query().Get("role")
	if role == "" {
		role = control.KeyRoleOwner // Default to owner
	}

	// Validate role
	validRoles := []string{control.KeyRoleOwner, control.KeyRoleChargingManager}
	isValid := false
	for _, validRole := range validRoles {
		if role == validRole {
			isValid = true
			break
		}
	}
	if !isValid {
		models.MainMessageStack.Push(models.Message{
			Title:   "Error",
			Message: fmt.Sprintf("Invalid role: %s", role),
			Type:    models.Error,
		})
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	err := control.CreatePrivateAndPublicKeyFileForRole(role)

	if err == nil {
		// Set as active key if no active key exists
		activeRole := control.GetActiveKeyRole()
		if activeRole == "" || !control.KeyExists(activeRole) {
			// Default to owner if active role is empty (shouldn't happen after migration, but just in case)
			if activeRole == "" {
				activeRole = control.KeyRoleOwner
			}
			if err := control.SetActiveKeyRole(role); err != nil {
				log.Warn("Failed to set active key role", "error", err)
			}
		}
		
		control.SetupBleControl()
		models.MainMessageStack.Push(models.Message{
			Title:   "Success",
			Message: fmt.Sprintf("Keys for role '%s' successfully generated and saved.", control.GetKeyRoleDisplayName(role)),
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
	// Get role from query parameter
	role := r.URL.Query().Get("role")
	
	// Role is required (legacy keys are automatically migrated)
	if role == "" {
		models.MainMessageStack.Push(models.Message{
			Title:   "Error",
			Message: "Role parameter is required. Legacy keys are automatically migrated to Owner role on startup.",
			Type:    models.Error,
		})
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	
	// Validate role
	validRoles := []string{control.KeyRoleOwner, control.KeyRoleChargingManager}
	isValid := false
	for _, validRole := range validRoles {
		if role == validRole {
			isValid = true
			break
		}
	}
	if !isValid {
		models.MainMessageStack.Push(models.Message{
			Title:   "Error",
			Message: fmt.Sprintf("Invalid role: %s. Valid roles are: owner, charging_manager", role),
			Type:    models.Error,
		})
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	
	// Remove role-based keys
	err1, err2 := control.RemoveKeyFilesForRole(role)
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
			Message: fmt.Sprintf("Keys for role '%s' successfully removed.", control.GetKeyRoleDisplayName(role)),
			Type:    models.Success,
		})
		
		// If removed key was active, try to activate another key
		if control.GetActiveKeyRole() == role {
				availableKeys := control.ListAvailableKeys()
				if len(availableKeys) > 0 {
					// Activate first available key (skip legacy/empty role if present)
					var newActiveRole string
					for _, key := range availableKeys {
						if key != "" {
							newActiveRole = key
							break
						}
					}
					// Fallback to owner if no valid role found
					if newActiveRole == "" {
						newActiveRole = control.KeyRoleOwner
					}
					if err := control.SetActiveKeyRole(newActiveRole); err == nil {
					models.MainMessageStack.Push(models.Message{
						Title:   "Info",
						Message: fmt.Sprintf("Active key changed to '%s'.", control.GetKeyRoleDisplayName(newActiveRole)),
						Type:    models.Info,
					})
				}
			} else {
				models.MainMessageStack.Push(models.Message{
					Title:   "Warning",
					Message: "No keys remaining. Please generate a new key to continue using the proxy.",
					Type:    models.Info,
				})
			}
		}
	}

	control.CloseBleControl()
	control.SetupBleControl() // Reinitialize with new active key
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
		role := r.FormValue("role")

		if vin == "" {
			models.MainMessageStack.Push(models.Message{
				Title:   "Error",
				Message: "No VIN entered.",
				Type:    models.Error,
			})
			return
		}

		// Validate role if provided
		if role != "" {
			validRoles := []string{control.KeyRoleOwner, control.KeyRoleChargingManager}
			isValid := false
			for _, validRole := range validRoles {
				if role == validRole {
					isValid = true
					break
				}
			}
			if !isValid {
				models.MainMessageStack.Push(models.Message{
					Title:   "Error",
					Message: fmt.Sprintf("Invalid role: %s. Valid roles are: owner, charging_manager", role),
					Type:    models.Error,
				})
				return
			}
			
			// Check if keys exist for this role
			if !control.KeyExists(role) {
				models.MainMessageStack.Push(models.Message{
					Title:   "Error",
					Message: fmt.Sprintf("Keys for role '%s' do not exist.", control.GetKeyRoleDisplayName(role)),
					Type:    models.Error,
				})
				return
			}
		} else {
			// Use active key role if no role specified
			activeRole := control.GetActiveKeyRole()
			// Ensure active role is not legacy (should have been migrated)
			if activeRole == "" {
				activeRole = control.KeyRoleOwner
			}
			role = activeRole
		}

		err := control.SendKeysToVehicle(vin, role)

		if err != nil {
			models.MainMessageStack.Push(models.Message{
				Title:   "Error",
				Message: err.Error(),
				Type:    models.Error,
			})
		} else {
			models.MainMessageStack.Push(models.Message{
				Title:   "Success",
				Message: fmt.Sprintf("Sent add-key request to %s with role '%s'. Confirm by tapping NFC card on center console.", vin, control.GetKeyRoleDisplayName(role)),
				Type:    models.Success,
			})
		}
	}
}

func ActivateKey(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		role := r.FormValue("role")

		// Validate role (legacy empty role is no longer valid)
		validRoles := []string{control.KeyRoleOwner, control.KeyRoleChargingManager}
		isValid := false
		for _, validRole := range validRoles {
			if role == validRole {
				isValid = true
				break
			}
		}
		if !isValid {
			models.MainMessageStack.Push(models.Message{
				Title:   "Error",
				Message: fmt.Sprintf("Invalid role: %s. Valid roles are: owner, charging_manager", role),
				Type:    models.Error,
			})
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}

		// Check if keys exist
		if !control.KeyExists(role) {
			models.MainMessageStack.Push(models.Message{
				Title:   "Error",
				Message: fmt.Sprintf("Keys for role '%s' do not exist.", control.GetKeyRoleDisplayName(role)),
				Type:    models.Error,
			})
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}

		if err := control.SetActiveKeyRole(role); err != nil {
			models.MainMessageStack.Push(models.Message{
				Title:   "Error",
				Message: err.Error(),
				Type:    models.Error,
			})
		} else {
			models.MainMessageStack.Push(models.Message{
				Title:   "Success",
				Message: fmt.Sprintf("Active key changed to '%s'.", control.GetKeyRoleDisplayName(role)),
				Type:    models.Success,
			})
			// Reinitialize BLE control with new active key
			control.CloseBleControl()
			control.SetupBleControl()
		}
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
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
