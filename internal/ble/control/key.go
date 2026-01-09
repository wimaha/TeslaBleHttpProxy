package control

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/wimaha/TeslaBleHttpProxy/config"
	"github.com/wimaha/TeslaBleHttpProxy/internal/tesla/commands"
)

func CreatePrivateAndPublicKeyFile() error {
	// Default to owner role for backward compatibility
	return CreatePrivateAndPublicKeyFileForRole(KeyRoleOwner)
}

func CreatePrivateAndPublicKeyFileForRole(role string) error {
	// Validate role to prevent path traversal
	validatedRole, err := ValidateRole(role)
	if err != nil {
		return fmt.Errorf("invalid role: %w", err)
	}
	role = validatedRole

	// Get file paths for the role
	privateKeyFile, publicKeyFile := GetKeyFiles(role)

	// For role-based keys, ensure directory exists
	if role != "" {
		keyDir := filepath.Dir(privateKeyFile)
		if err := os.MkdirAll(keyDir, 0755); err != nil {
			log.Error("Error creating key directory", "Error", err, "Directory", keyDir)
			return err
		}
	}

	// Check if keys already exist
	if _, err := os.Stat(privateKeyFile); err == nil {
		return fmt.Errorf("keys for role '%s' already exist", GetKeyRoleDisplayName(role))
	}

	// Generate ECDSA private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Error("Error generating ECDSA private key", "Error", err)
		return err
	}

	// Encode the private key to PEM format
	x509Encoded, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		log.Error("Error encoding ECDSA private key", "Error", err)
		return err
	}
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: x509Encoded})

	// Write the PEM-encoded private key to a file
	privFile, err := os.Create(privateKeyFile)
	if err != nil {
		log.Error("Error creating private key file", "Error", err, "File", privateKeyFile)
		return err
	}
	defer privFile.Close()

	// Set restrictive permissions on private key (0600 = owner read/write only)
	if err := privFile.Chmod(0600); err != nil {
		log.Warn("Failed to set private key permissions", "Error", err)
	}

	_, err = privFile.Write(pemEncoded)
	if err != nil {
		log.Error("Error writing to private key file", "Error", err)
		return err
	}

	log.Info("ECDSA private key generated and saved", "Role", GetKeyRoleDisplayName(role), "File", privateKeyFile)

	// Extract the public key from the private key
	publicKey := &privateKey.PublicKey

	// Encode the public key to PEM format
	x509EncodedPub, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		log.Error("Error encoding ECDSA public key", "Error", err)
		return err
	}
	pemEncodedPublicKey := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: x509EncodedPub})

	// Write the PEM-encoded public key to a file
	pubFile, err := os.Create(publicKeyFile)
	if err != nil {
		log.Error("Error creating public key file", "Error", err, "File", publicKeyFile)
		return err
	}
	defer pubFile.Close()

	_, err = pubFile.Write(pemEncodedPublicKey)
	if err != nil {
		log.Error("Error writing to public key file", "Error", err)
		return err
	}

	log.Info("ECDSA public key generated and saved", "Role", GetKeyRoleDisplayName(role), "File", publicKeyFile)

	return nil
}

func RemoveKeyFiles() (error, error) {
	// Remove legacy keys for backward compatibility
	err1 := os.Remove(config.PrivateKeyFile)
	err2 := os.Remove(config.PublicKeyFile)
	return err1, err2
}

func SendKeysToVehicle(vin string, role string) error {
	tempBleControl := &BleControl{
		privateKey:   nil,
		commandStack: make(chan commands.Command, 1),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := &commands.Command{
		Command: "add-key-request",
		Vin:     vin,
		Body:    map[string]interface{}{"role": role},
	}
	conn, car, _, err := tempBleControl.TryConnectToVehicle(ctx, cmd)
	if err == nil {
		//Successful
		defer conn.Close()
		defer log.Debug("close connection (A)")
		defer car.Disconnect()
		defer log.Debug("disconnect vehicle (A)")

		_, err, _ := tempBleControl.ExecuteCommand(car, cmd, context.Background())
		if err != nil {
			return err
		}

		return nil
	} else {
		return err
	}
}
