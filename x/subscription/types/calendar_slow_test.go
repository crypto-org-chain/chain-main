// +build slowtest

package types_test

import (
	"testing"
	"time"
)

func TestSecsToTMSlow(t *testing.T) {
	time.Local = time.UTC
	var i int64
	for i = 0; i < 100000000; i++ {
		TimestampTest(t, i)
	}
}
