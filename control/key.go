package control

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"time"

	"github.com/charmbracelet/log"
)

func CreatePrivateAndPublicKeyFile() error {
	// Generate ECDSA private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Error("error generating ECDSA private key", "err", err)
		return err
	}

	// Encode the private key to PEM format
	x509Encoded, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		log.Error("error encoding ECDSA private key", "err", err)
		return err
	}
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: x509Encoded})

	// Write the PEM-encoded private key to a file
	privateKeyFile, err := os.Create(PrivateKeyFile)
	if err != nil {
		log.Error("Error creating private key file", "err", err)
		return err
	}
	defer privateKeyFile.Close()

	_, err = privateKeyFile.Write(pemEncoded)
	if err != nil {
		log.Error("Error writing to private key file", "err", err)
		return err
	}

	log.Info("ECDSA private key generated and saved")

	// Extract the public key from the private key
	publicKey := &privateKey.PublicKey

	// Encode the public key to PEM format
	x509EncodedPub, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		log.Error("Error encoding ECDSA public key", "err", err)
		return err
	}
	pemEncodedPublicKey := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: x509EncodedPub})

	// Write the PEM-encoded public key to a file
	publicKeyFile, err := os.Create(PublicKeyFile)
	if err != nil {
		log.Error("Error creating public key file", "err", err)
		return err
	}
	defer publicKeyFile.Close()

	_, err = publicKeyFile.Write(pemEncodedPublicKey)
	if err != nil {
		log.Error("Error writing to public key file", "err", err)
		return err
	}

	log.Info("ECDSA public key generated and saved")

	return nil
}

func RemoveKeyFiles() (error, error) {
	err1 := os.Remove(PrivateKeyFile)
	err2 := os.Remove(PublicKeyFile)

	return err1, err2
}

func SendKeysToVehicle(vin string) error {
	tempBleControl := &BleControl{
		privateKey:   nil,
		commandStack: make(chan Command, 1),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := &Command{
		Command: "add-key-request",
		Vin:     vin,
	}
	conn, car, _, err := tempBleControl.tryConnectToVehicle(ctx, cmd)
	if err == nil {
		//Successful
		defer conn.Close()
		defer log.Debug("close connection (A)")
		defer car.Disconnect()
		defer log.Debug("disconnect vehicle (A)")

		_, err := tempBleControl.executeCommand(car, cmd)
		if err != nil {
			return err
		}

		return nil
	} else {
		return err
	}
}
