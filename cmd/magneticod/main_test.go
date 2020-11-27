// +build linux

package main

import (
	"testing"

	"github.com/Wessie/appdirs"
)

func TestAppdirs(t *testing.T) {
	var expected, returned string

	returned = appdirs.UserDataDir("magneticod", "", "", false)
	expected = appdirs.ExpandUser("~/.local/share/magneticod")
	if returned != expected {
		t.Errorf("UserDataDir returned an unexpected value!  `%s`", returned)
	}

	returned = appdirs.UserCacheDir("magneticod", "", "", true)
	expected = appdirs.ExpandUser("~/.cache/magneticod")
	if returned != expected {
		t.Errorf("UserCacheDir returned an unexpected value!  `%s`", returned)
	}
}
