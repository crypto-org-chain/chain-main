package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cosmos/gogoproto/proto"
	"github.com/spf13/cobra"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type validatingMsg interface {
	sdk.Msg
	Validate() error
}

func readJSONArgOrFile(arg string) ([]byte, error) {
	trimmed := strings.TrimSpace(arg)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return []byte(trimmed), nil
	}

	bz, err := os.ReadFile(filepath.Clean(arg))
	if err == nil {
		return bz, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}

	if looksLikeFilePath(trimmed) {
		return nil, fmt.Errorf("file not found: %s", arg)
	}

	return []byte(arg), nil
}

func looksLikeFilePath(s string) bool {
	return strings.ContainsAny(s, "/\\") || strings.HasSuffix(s, ".json")
}

func unmarshalJSONArg(clientCtx client.Context, arg string, dst proto.Message) error {
	bz, err := readJSONArgOrFile(arg)
	if err != nil {
		return err
	}

	return clientCtx.Codec.UnmarshalJSON(bz, dst)
}

func parseUint32Arg(name, value string) (uint32, error) {
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, value, err)
	}

	return uint32(parsed), nil
}

func parseUint64Arg(name, value string) (uint64, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, value, err)
	}

	return parsed, nil
}

func parseMathIntArg(name, value string) (sdkmath.Int, error) {
	parsed, ok := sdkmath.NewIntFromString(value)
	if !ok {
		return sdkmath.Int{}, fmt.Errorf("invalid %s %q", name, value)
	}

	return parsed, nil
}

func mustMarkFlagRequired(cmd *cobra.Command, name string) {
	if err := cmd.MarkFlagRequired(name); err != nil {
		panic(err)
	}
}
