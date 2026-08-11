package relayclient

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Credentials struct {
	PanelID string `json:"panelId"`
	Secret  string `json:"secret"`
}

func LoadOrCreateCredentials(path string) (Credentials, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		var credentials Credentials
		if json.Unmarshal(data, &credentials) != nil || !validCredential(credentials.PanelID) || !validCredential(credentials.Secret) {
			return Credentials{}, errors.New("invalid FaroOS relay credentials file")
		}
		return credentials, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Credentials{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return Credentials{}, err
	}
	panelID, err := randomCredential()
	if err != nil {
		return Credentials{}, err
	}
	secret, err := randomCredential()
	if err != nil {
		return Credentials{}, err
	}
	credentials := Credentials{PanelID: panelID, Secret: secret}
	data, err = json.Marshal(credentials)
	if err != nil {
		return Credentials{}, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return LoadOrCreateCredentials(path)
	}
	if err != nil {
		return Credentials{}, err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return Credentials{}, err
	}
	if err := file.Close(); err != nil {
		return Credentials{}, err
	}
	return credentials, nil
}

func validCredential(value string) bool {
	data, err := base64.RawURLEncoding.DecodeString(value)
	return err == nil && len(data) >= 24
}

func randomCredential() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
