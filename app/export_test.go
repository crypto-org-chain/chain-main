package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExportAppStateAndValidators(t *testing.T) {
	testCases := []struct {
		name          string
		forZeroHeight bool
	}{
		{
			"for zero height",
			true,
		},
		{
			"for non-zero height",
			false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := Setup(t, false)
			app.Commit()
			_, err := app.ExportAppStateAndValidators(tc.forZeroHeight, []string{})
			require.NoError(t, err, "ExportAppStateAndValidators should not have an error")
		})
	}
}
