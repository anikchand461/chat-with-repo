package api

import (
	"os"
	"path/filepath"
)

// appDir is the app's private storage directory. os.UserConfigDir()
// resolves to it on Android when built with gogio, and to the usual
// per-OS config dir elsewhere.
func appDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	appDir := filepath.Join(dir, "chatwithrepo")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return "", err
	}
	return appDir, nil
}

// tokenFile returns the path used to persist the access token on-device.
func tokenFile() (string, error) {
	dir, err := appDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "token"), nil
}

// SaveToken persists the access token so the user stays logged in
// between app launches.
func SaveToken(token string) error {
	path, err := tokenFile()
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(token), 0o600)
}

// LoadToken reads a previously saved token, if any. It returns an empty
// string (no error) when nothing has been saved yet.
func LoadToken() (string, error) {
	path, err := tokenFile()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// ClearToken removes the saved token, e.g. on logout or 401.
func ClearToken() error {
	path, err := tokenFile()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
