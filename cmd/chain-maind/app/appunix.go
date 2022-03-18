//go:build linux || darwin
// +build linux darwin

package app

import (
	"os"

	"github.com/google/renameio"
)

func WriteFile(filename string, data []byte, perm os.FileMode) error {
	return renameio.WriteFile(filename, data, perm)
}
