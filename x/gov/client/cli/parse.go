package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/cosmos/cosmos-sdk/x/gov/client/cli"
	govutils "github.com/cosmos/cosmos-sdk/x/gov/client/utils"
	"github.com/spf13/pflag"
)

func parseSubmitProposalFlags(fs *pflag.FlagSet) (*proposal, error) {
	// err/nil checking handled in flag processing
	proposal := &proposal{}
	// nolint: errcheck
	proposalFile, _ := fs.GetString(cli.FlagProposal)

	if proposalFile == "" {
		// nolint: errcheck
		proposalType, _ := fs.GetString(cli.FlagProposalType)

		// nolint: errcheck
		proposal.Title, _ = fs.GetString(cli.FlagTitle)
		// nolint: errcheck
		proposal.Description, _ = fs.GetString(cli.FlagDescription)
		proposal.Type = govutils.NormalizeProposalType(proposalType)
		// nolint: errcheck
		proposal.Deposit, _ = fs.GetString(cli.FlagDeposit)
		return proposal, nil
	}

	for _, flag := range cli.ProposalFlags {
		// nolint: errcheck
		if v, _ := fs.GetString(flag); v != "" {
			return nil, fmt.Errorf("--%s flag provided alongside --proposal, which is a noop", flag)
		}
	}

	contents, err := ioutil.ReadFile(proposalFile)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(contents, proposal)
	if err != nil {
		return nil, err
	}

	return proposal, nil
}
