package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
)

// avatarImage mirrors GET /auth/avatar's response; encoding/json reads
// the base64 "data" field straight into bytes.
type avatarImage struct {
	ContentType string `json:"content_type"`
	Data        []byte `json:"data"`
}

type avatarUpload struct {
	Image []byte `json:"image"` // sent as base64
}

type avatarInfo struct {
	Avatar string `json:"avatar"`
}

// Avatar downloads the profile picture the user uploaded (JPEG or PNG).
func (c *Client) Avatar() ([]byte, error) {
	var out avatarImage
	if err := c.do(http.MethodGet, "/auth/avatar", nil, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// UploadAvatar replaces the profile picture with image (JPEG or PNG, at
// most 512 KB). Returns the new avatar source ("custom").
func (c *Client) UploadAvatar(image []byte) (string, error) {
	var out avatarInfo
	if err := c.do(http.MethodPost, "/auth/avatar", avatarUpload{Image: image}, &out); err != nil {
		return "", err
	}
	return out.Avatar, nil
}

// DeleteAvatar removes the uploaded picture. Returns what shows instead:
// "google" or "default".
func (c *Client) DeleteAvatar() (string, error) {
	var out avatarInfo
	if err := c.do(http.MethodDelete, "/auth/avatar", nil, &out); err != nil {
		return "", err
	}
	return out.Avatar, nil
}

// googleSize matches the size suffix of Google profile photo URLs
// (".../photo=s96-c").
var googleSize = regexp.MustCompile(`=s\d+-c$`)

// GooglePicture downloads the Google profile photo at url, asking Google
// for a 256 px version so it stays sharp on high-density screens.
func (c *Client) GooglePicture(url string) ([]byte, error) {
	resp, err := c.http.Get(googleSize.ReplaceAllString(url, "=s256-c"))
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("couldn't load the Google profile photo")
	}
	return io.ReadAll(io.LimitReader(resp.Body, 2<<20))
}
