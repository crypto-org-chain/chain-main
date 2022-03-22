//go:build windows
// +build windows

package app

import (
	"io/ioutil"
	"os"
)

func WriteFile(filename string, data []byte, perm os.FileMode) error {
	return ioutil.WriteFile(filename, data, perm)
}
