package key

import (
	"fmt"
	"io/ioutil"
	"os"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type PrivateKeyNotFoundErr struct {
	filePath string
}

func (e PrivateKeyNotFoundErr) Error() string {
	return fmt.Sprintf("no private key file found at %s", e.filePath)
}

func IsPrivateKeyNotFound(err error) bool {
	_, ok := err.(PrivateKeyNotFoundErr)
	return ok
}

func LoadPrivateKey(filePath string) (*wgtypes.Key, error) {
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return nil, PrivateKeyNotFoundErr{}
	}

	b, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read private key: %w", err)
	}

	privateKey, err := wgtypes.ParseKey(string(b))
	if err != nil {
		return nil, fmt.Errorf("unable to read parse private key: %w", err)
	}

	return &privateKey, nil
}

func GenerateKey(filePath string) (*wgtypes.Key, error) {
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("unable to generate key: %w", err)
	}

	if err := ioutil.WriteFile(filePath, []byte(privateKey.String()), 0400); err != nil {
		return nil, fmt.Errorf("unable to write private key: %w", err)
	}

	return &privateKey, nil
}
