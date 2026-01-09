package control

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
)

const (
	KeyRoleOwner           = "owner"
	KeyRoleChargingManager = "charging_manager"
)

const activeKeyConfigFile = "key/active_key.json"

type ActiveKeyConfig struct {
	Role string `json:"role"`
}

// GetKeyFiles returns the private and public key file paths for a given role
func GetKeyFiles(role string) (privateKeyFile, publicKeyFile string) {
	// Support legacy single key format for backward compatibility
	if role == "" {
		// Check if legacy keys exist
		if _, err := os.Stat("key/private.pem"); err == nil {
			// If owner keys exist (migration happened), prefer them
			if _, err := os.Stat("key/owner/private.pem"); err == nil {
				// Owner keys exist, use them instead of legacy
				return "key/owner/private.pem", "key/owner/public.pem"
			}
			return "key/private.pem", "key/public.pem"
		}
		// Default to owner if no legacy keys
		role = KeyRoleOwner
	}

	// New role-based key structure
	return fmt.Sprintf("key/%s/private.pem", role), fmt.Sprintf("key/%s/public.pem", role)
}

// GetActiveKeyRole returns the currently active key role
func GetActiveKeyRole() string {
	// Check for active key config file
	if data, err := os.ReadFile(activeKeyConfigFile); err == nil {
		var config ActiveKeyConfig
		if err := json.Unmarshal(data, &config); err == nil && config.Role != "" {
			return config.Role
		}
	}

	// Legacy keys should have been migrated automatically on startup
	// If legacy keys still exist, try to use owner (migration might be pending or failed)
	if _, err := os.Stat("key/private.pem"); err == nil {
		// Check if owner keys exist (migration should have created them)
		if _, err := os.Stat("key/owner/private.pem"); err == nil {
			// Owner keys exist, use them
			return KeyRoleOwner
		}
		// Legacy keys exist but owner doesn't - migration might have failed
		// Return owner anyway as that's where they should be
		log.Warn("Legacy keys detected but owner keys not found. Migration may have failed.")
		return KeyRoleOwner
	}

	// Default to owner if no keys exist
	return KeyRoleOwner
}

// SetActiveKeyRole sets the active key role
func SetActiveKeyRole(role string) error {
	// Validate role (legacy empty role is no longer valid - should be migrated)
	if role == "" {
		return fmt.Errorf("invalid role: empty role. Legacy keys should be migrated to owner role")
	}

	validRoles := []string{KeyRoleOwner, KeyRoleChargingManager}
	isValid := false
	for _, validRole := range validRoles {
		if role == validRole {
			isValid = true
			break
		}
	}
	if !isValid {
		return fmt.Errorf("invalid role: %s. Valid roles are: owner, charging_manager", role)
	}

	// Check if keys exist for this role
	privateKeyFile, _ := GetKeyFiles(role)
	if _, err := os.Stat(privateKeyFile); err != nil {
		return fmt.Errorf("keys for role '%s' do not exist", role)
	}

	// Create config
	config := ActiveKeyConfig{Role: role}
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Ensure key directory exists
	if err := os.MkdirAll("key", 0755); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	// Write config file
	if err := os.WriteFile(activeKeyConfigFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write active key config: %w", err)
	}

	log.Info("Active key role set", "role", role)
	return nil
}

// ListAvailableKeys returns a list of all available key roles
func ListAvailableKeys() []string {
	var roles []string

	// Check for legacy keys (should be migrated, but check for backward compatibility)
	if _, err := os.Stat("key/private.pem"); err == nil {
		// Only include legacy if owner keys don't exist (migration might be pending)
		if _, err := os.Stat("key/owner/private.pem"); err != nil {
			roles = append(roles, "")
		}
	}

	// Check role-based keys
	validRoles := []string{KeyRoleOwner, KeyRoleChargingManager}
	for _, role := range validRoles {
		privateKeyFile, _ := GetKeyFiles(role)
		if _, err := os.Stat(privateKeyFile); err == nil {
			roles = append(roles, role)
		}
	}

	return roles
}

// GetKeyRoleDisplayName returns a display name for a role
func GetKeyRoleDisplayName(role string) string {
	if role == "" {
		return "Legacy (Owner)"
	}
	switch role {
	case KeyRoleOwner:
		return "Owner"
	case KeyRoleChargingManager:
		return "Charging Manager"
	default:
		return role
	}
}

// KeyExists checks if keys exist for a given role
func KeyExists(role string) bool {
	privateKeyFile, _ := GetKeyFiles(role)
	_, err := os.Stat(privateKeyFile)
	return err == nil
}

// RemoveKeyFiles removes keys for a specific role
func RemoveKeyFilesForRole(role string) (error, error) {
	privateKeyFile, publicKeyFile := GetKeyFiles(role)

	var err1, err2 error

	// Remove private key
	if _, err := os.Stat(privateKeyFile); err == nil {
		err1 = os.Remove(privateKeyFile)
	}

	// Remove public key
	if _, err := os.Stat(publicKeyFile); err == nil {
		err2 = os.Remove(publicKeyFile)
	}

	// If role-based, try to remove directory if empty
	if role != "" {
		keyDir := filepath.Dir(privateKeyFile)
		if entries, err := os.ReadDir(keyDir); err == nil && len(entries) == 0 {
			os.Remove(keyDir)
		}
	}

	return err1, err2
}

// MigrateLegacyKeys automatically migrates legacy keys to the owner role structure
func MigrateLegacyKeys() error {
	// Check if legacy keys exist
	legacyPrivateKey := "key/private.pem"
	legacyPublicKey := "key/public.pem"

	if _, err := os.Stat(legacyPrivateKey); err != nil {
		// No legacy keys to migrate
		return nil
	}

	// Check if owner keys already exist (don't overwrite)
	ownerPrivateKey, ownerPublicKey := GetKeyFiles(KeyRoleOwner)
	if _, err := os.Stat(ownerPrivateKey); err == nil {
		// Owner keys already exist, don't migrate
		log.Info("Legacy keys detected but owner keys already exist. Skipping migration.")
		return nil
	}

	log.Info("Migrating legacy keys to owner role structure...")

	// Ensure owner directory exists
	ownerDir := filepath.Dir(ownerPrivateKey)
	if err := os.MkdirAll(ownerDir, 0755); err != nil {
		return fmt.Errorf("failed to create owner key directory: %w", err)
	}

	// Read legacy keys
	privateKeyData, err := os.ReadFile(legacyPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to read legacy private key: %w", err)
	}

	publicKeyData, err := os.ReadFile(legacyPublicKey)
	if err != nil {
		return fmt.Errorf("failed to read legacy public key: %w", err)
	}

	// Write to owner role location
	ownerPrivFile, err := os.Create(ownerPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to create owner private key file: %w", err)
	}
	defer ownerPrivFile.Close()

	// Set restrictive permissions on private key
	if err := ownerPrivFile.Chmod(0600); err != nil {
		log.Warn("Failed to set owner private key permissions", "Error", err)
	}

	if _, err := ownerPrivFile.Write(privateKeyData); err != nil {
		return fmt.Errorf("failed to write owner private key: %w", err)
	}

	ownerPubFile, err := os.Create(ownerPublicKey)
	if err != nil {
		return fmt.Errorf("failed to create owner public key file: %w", err)
	}
	defer ownerPubFile.Close()

	if _, err := ownerPubFile.Write(publicKeyData); err != nil {
		return fmt.Errorf("failed to write owner public key: %w", err)
	}

	// Set active key to owner
	if err := SetActiveKeyRole(KeyRoleOwner); err != nil {
		return fmt.Errorf("failed to set active key role: %w", err)
	}

	// Remove legacy keys after successful migration
	if err := os.Remove(legacyPrivateKey); err != nil {
		log.Warn("Failed to remove legacy private key after migration", "Error", err)
	}
	if err := os.Remove(legacyPublicKey); err != nil {
		log.Warn("Failed to remove legacy public key after migration", "Error", err)
	}

	log.Info("Legacy keys successfully migrated to owner role")
	return nil
}
