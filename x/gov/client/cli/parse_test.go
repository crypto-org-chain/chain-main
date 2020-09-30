package cli

import (
	"testing"

	"github.com/crypto-com/chain-main/test"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/gov/client/cli"
)

func TestParseSubmitProposalFlags(t *testing.T) {
	fakeCoinParser := test.NewFakeCoinParser(map[string]sdk.Coin{
		"1000test": sdk.NewCoin("test", sdk.NewInt(1000)),
	})

	okJSON, cleanup1 := testutil.WriteToNewTempFile(t, `
{
  "title": "Test Proposal",
  "description": "My awesome proposal",
  "type": "Text",
  "deposit": "1000test"
}
`)
	t.Cleanup(cleanup1)

	badJSON, cleanup2 := testutil.WriteToNewTempFile(t, "bad json")
	t.Cleanup(cleanup2)

	fs := NewCmdSubmitProposal(fakeCoinParser).Flags()

	// nonexistent json
	fs.Set(cli.FlagProposal, "fileDoesNotExist") // nolint: errcheck
	_, err := parseSubmitProposalFlags(fs)
	require.Error(t, err)

	// invalid json
	fs.Set(cli.FlagProposal, badJSON.Name()) // nolint: errcheck
	_, err = parseSubmitProposalFlags(fs)
	require.Error(t, err)

	// ok json
	fs.Set(cli.FlagProposal, okJSON.Name()) // nolint: errcheck
	proposal1, err := parseSubmitProposalFlags(fs)
	require.Nil(t, err, "unexpected error")
	require.Equal(t, "Test Proposal", proposal1.Title)
	require.Equal(t, "My awesome proposal", proposal1.Description)
	require.Equal(t, "Text", proposal1.Type)
	require.Equal(t, "1000test", proposal1.Deposit)

	// flags that can't be used with --proposal
	for _, incompatibleFlag := range cli.ProposalFlags {
		fs.Set(incompatibleFlag, "some value") // nolint: errcheck
		_, err2 := parseSubmitProposalFlags(fs)
		require.Error(t, err2)
		fs.Set(incompatibleFlag, "") // nolint: errcheck
	}

	// no --proposal, only flags
	fs.Set(cli.FlagProposal, "")                       // nolint: errcheck
	fs.Set(cli.FlagTitle, proposal1.Title)             // nolint: errcheck
	fs.Set(cli.FlagDescription, proposal1.Description) // nolint: errcheck
	fs.Set(cli.FlagProposalType, proposal1.Type)       // nolint: errcheck
	fs.Set(cli.FlagDeposit, proposal1.Deposit)         // nolint: errcheck
	proposal2, err := parseSubmitProposalFlags(fs)

	require.Nil(t, err, "unexpected error")
	require.Equal(t, proposal1.Title, proposal2.Title)
	require.Equal(t, proposal1.Description, proposal2.Description)
	require.Equal(t, proposal1.Type, proposal2.Type)
	require.Equal(t, proposal1.Deposit, proposal2.Deposit)

	err = okJSON.Close()
	require.Nil(t, err, "unexpected error")
	err = badJSON.Close()
	require.Nil(t, err, "unexpected error")
}
