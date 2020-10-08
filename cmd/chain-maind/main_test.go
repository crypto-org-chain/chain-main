// +build testbincover

package main

import (
	"testing"

	"github.com/confluentinc/bincover"
)

// TestBincoverRunMain wrap main in test function to have coverage support
// https://www.confluent.io/blog/measure-go-code-coverage-with-bincover/
func TestBincoverRunMain(t *testing.T) {
	bincover.RunTest(main)
}
