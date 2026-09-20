//go:build !android

package ui

import "errors"

func openURLAndroid(string) error { return errors.New("not running on Android") }
