package control

import (
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/wimaha/TeslaBleHttpProxy/internal/logging"
)

// LoadPrivateKey loads a private key from file (protected by UNIX file permissions)
func LoadPrivateKey(privateKeyFile string) (protocol.ECDHPrivateKey, error) {
	// Load using protocol's loader - file permissions (0600) protect the key
	privateKey, err := protocol.LoadPrivateKey(privateKeyFile)
	if err != nil {
		return nil, err
	}
	logging.Debug("Private key loaded", "File", privateKeyFile)
	return privateKey, nil
}
