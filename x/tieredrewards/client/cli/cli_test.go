package cli_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/cosmos/gogoproto/proto"
	"github.com/crypto-org-chain/chain-main/v8/testutil"
	tieredrewardstestutil "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/client/testutil"
	tieredrewardstypes "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"
	"github.com/stretchr/testify/suite"

	sdkmath "cosmossdk.io/math"

	sdktestutil "github.com/cosmos/cosmos-sdk/testutil"
	"github.com/cosmos/cosmos-sdk/testutil/network"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

type IntegrationTestSuite struct {
	suite.Suite

	cfg     network.Config
	network *network.Network
}

func (s *IntegrationTestSuite) SetupSuite() {
	var err error

	cfg := network.DefaultConfig(tieredrewardstestutil.NewChainMainTestNetworkFixture)
	cfg.ChainID = testutil.ChainID
	cfg.AppConstructor = tieredrewardstestutil.GetApp
	cfg.NumValidators = 2
	cfg.TimeoutCommit = 200 * time.Millisecond

	s.cfg = cfg
	s.network, err = network.New(s.T(), s.T().TempDir(), cfg)
	s.Require().NoError(err)

	_, err = s.network.WaitForHeight(1)
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.network.Cleanup()
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (s *IntegrationTestSuite) defaultTxArgs() []string {
	return tieredrewardstestutil.DefaultTxArgs(s.cfg.BondDenom)
}

func (s *IntegrationTestSuite) proposalArgs(title, summary, deposit, metadata string) []string {
	args := []string{
		fmt.Sprintf("--title=%s", title),
		fmt.Sprintf("--summary=%s", summary),
		fmt.Sprintf("--deposit=%s", deposit),
	}
	if metadata != "" {
		args = append(args, fmt.Sprintf("--metadata=%s", metadata))
	}

	return append(args, s.defaultTxArgs()...)
}

func (s *IntegrationTestSuite) waitBlocks(count int) {
	for range count {
		s.Require().NoError(s.network.WaitForNextBlock())
	}
}

func (s *IntegrationTestSuite) mustExecQuery(val *network.Validator, exec func() (sdktestutil.BufferWriter, error), resp proto.Message) {
	bz, err := exec()
	s.Require().NoError(err)
	s.Require().NoError(val.ClientCtx.Codec.UnmarshalJSON(bz.Bytes(), resp), bz.String())
}

func (s *IntegrationTestSuite) mustQueryPosition(val *network.Validator, positionID string) tieredrewardstypes.PositionResponse {
	var resp tieredrewardstypes.QueryTierPositionResponse
	s.mustExecQuery(val, func() (sdktestutil.BufferWriter, error) {
		return tieredrewardstestutil.QueryTierPositionExec(val.ClientCtx, positionID)
	}, &resp)
	return resp.Position
}

func (s *IntegrationTestSuite) mustQueryRewardsPoolBalances(val *network.Validator) sdk.Coins {
	var resp tieredrewardstypes.QueryRewardsPoolBalancesResponse
	s.mustExecQuery(val, func() (sdktestutil.BufferWriter, error) {
		return tieredrewardstestutil.QueryRewardsPoolBalancesExec(val.ClientCtx)
	}, &resp)
	return resp.Balances
}

func (s *IntegrationTestSuite) mustQueryAllPositions(val *network.Validator, extraArgs ...string) tieredrewardstypes.QueryAllTierPositionsResponse {
	var resp tieredrewardstypes.QueryAllTierPositionsResponse
	s.mustExecQuery(val, func() (sdktestutil.BufferWriter, error) {
		return tieredrewardstestutil.QueryAllTierPositionsExec(val.ClientCtx, extraArgs...)
	}, &resp)
	return resp
}

func (s *IntegrationTestSuite) assertCLIError(exec func() (sdktestutil.BufferWriter, error), wantContains string) {
	_, err := exec()
	s.Require().Error(err)
	if wantContains != "" {
		s.Require().Contains(err.Error(), wantContains)
	}
}

func (s *IntegrationTestSuite) mustExecTx(val *network.Validator, exec func() (sdktestutil.BufferWriter, error)) sdk.TxResponse {
	bz, err := exec()
	s.Require().NoError(err)

	var txResp sdk.TxResponse
	s.Require().NoError(val.ClientCtx.Codec.UnmarshalJSON(bz.Bytes(), &txResp), bz.String())
	s.Require().Zero(txResp.Code, bz.String())

	txClient := txtypes.NewServiceClient(val.ClientCtx)
	for range 3 {
		s.Require().NoError(s.network.WaitForNextBlock())
		grpcResp, err := txClient.GetTx(context.Background(), &txtypes.GetTxRequest{Hash: txResp.TxHash})
		if err == nil {
			s.Require().NotNil(grpcResp.TxResponse)
			s.Require().Zero(grpcResp.TxResponse.Code, grpcResp.TxResponse.RawLog)
			return *grpcResp.TxResponse
		}
	}
	s.FailNow("tx not queryable after 3 blocks", txResp.TxHash)
	return sdk.TxResponse{}
}

func (s *IntegrationTestSuite) proposalIDFromTx(txResp sdk.TxResponse) uint64 {
	for _, event := range txResp.Events {
		for _, attr := range event.Attributes {
			if attr.Key == "proposal_id" {
				proposalID, err := strconv.ParseUint(attr.Value, 10, 64)
				s.Require().NoError(err)
				return proposalID
			}
		}
	}

	s.FailNow("proposal_id event attribute not found")
	return 0
}

func (s *IntegrationTestSuite) mustGetProposal(val *network.Validator, proposalID uint64) *v1.Proposal {
	govQueryClient := v1.NewQueryClient(val.ClientCtx)
	resp, err := govQueryClient.Proposal(context.Background(), &v1.QueryProposalRequest{ProposalId: proposalID})
	s.Require().NoError(err)
	s.Require().NotNil(resp.Proposal)
	return resp.Proposal
}

func (s *IntegrationTestSuite) mustGetTxMsg(val *network.Validator, txHash string) sdk.Msg {
	txQueryClient := txtypes.NewServiceClient(val.ClientCtx)
	resp, err := txQueryClient.GetTx(context.Background(), &txtypes.GetTxRequest{Hash: txHash})
	s.Require().NoError(err)
	s.Require().NotNil(resp.Tx)
	s.Require().NotNil(resp.Tx.Body)
	s.Require().NotEmpty(resp.Tx.Body.Messages)

	var msg sdk.Msg
	err = val.ClientCtx.InterfaceRegistry.UnpackAny(resp.Tx.Body.Messages[0], &msg)
	s.Require().NoError(err)

	return msg
}

func (s *IntegrationTestSuite) mustUnpackProposalMsg(val *network.Validator, proposal *v1.Proposal) sdk.Msg {
	s.Require().Len(proposal.Messages, 1)

	var msg sdk.Msg
	err := val.ClientCtx.InterfaceRegistry.UnpackAny(proposal.Messages[0], &msg)
	s.Require().NoError(err)

	return msg
}

func (s *IntegrationTestSuite) writeTempJSON(name, contents string) string {
	path := filepath.Join(s.T().TempDir(), name)
	s.Require().NoError(os.WriteFile(path, []byte(contents), 0o600))
	return path
}

func (s *IntegrationTestSuite) TestTieredRewardsCLI() {
	val := s.network.Validators[0]
	val2 := s.network.Validators[1]

	owner := val.Address.String()
	validator := val.ValAddress.String()
	dstValidator := val2.ValAddress.String()

	lockAmount := sdkmath.NewInt(30_000_000)
	addAmount := sdkmath.NewInt(10_000_000)
	commitAmount := sdkmath.NewInt(10_000_000)
	exitImmediatelyLockAmount := sdkmath.NewInt(10_000_000)
	deposit := sdk.NewCoin(s.cfg.BondDenom, sdkmath.OneInt()).String()
	expeditedDeposit := sdk.NewCoin(s.cfg.BondDenom, sdkmath.NewInt(2)).String()

	s.Run("genesis queries", func() {
		var paramsResp tieredrewardstypes.QueryParamsResponse
		s.mustExecQuery(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.QueryParamsExec(val.ClientCtx)
		}, &paramsResp)
		s.Require().True(paramsResp.Params.TargetBaseRewardsRate.Equal(sdkmath.LegacyZeroDec()))

		var tiersResp tieredrewardstypes.QueryTiersResponse
		s.mustExecQuery(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.QueryTiersExec(val.ClientCtx)
		}, &tiersResp)
		s.Require().Len(tiersResp.Tiers, 1)
		s.Require().Equal(tieredrewardstestutil.TestTierID, tiersResp.Tiers[0].Id)
		s.Require().Equal(tieredrewardstestutil.TestExitDuration, tiersResp.Tiers[0].ExitDuration)
		s.Require().True(tiersResp.Tiers[0].BonusApy.Equal(sdkmath.LegacyOneDec()))

		poolBalance := s.mustQueryRewardsPoolBalances(val)
		s.Require().True(poolBalance.AmountOf(s.cfg.BondDenom).IsPositive(), "rewards pool should be funded at genesis")
	})

	s.Run("governance proposals", func() {
		updateParamsTx := s.mustExecTx(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.UpdateParamsProposalExec(
				val.ClientCtx,
				owner,
				`{"target_base_rewards_rate":"0.03"}`,
				s.proposalArgs("update tieredrewards params", "ensure params JSON is parsed", deposit, "tieredrewards-update-params")...,
			)
		})
		updateParamsProposal := s.mustGetProposal(val, s.proposalIDFromTx(updateParamsTx))
		s.Require().Equal("update tieredrewards params", updateParamsProposal.Title)
		s.Require().Equal("ensure params JSON is parsed", updateParamsProposal.Summary)
		s.Require().Equal("tieredrewards-update-params", updateParamsProposal.Metadata)
		updateParamsMsg, ok := s.mustUnpackProposalMsg(val, updateParamsProposal).(*tieredrewardstypes.MsgUpdateParams)
		s.Require().True(ok)
		s.Require().True(updateParamsMsg.Params.TargetBaseRewardsRate.Equal(sdkmath.LegacyMustNewDecFromStr("0.03")))

		expeditedProposalTx := s.mustExecTx(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.UpdateParamsProposalExec(
				val.ClientCtx,
				owner,
				`{"target_base_rewards_rate":"0.04"}`,
				append(
					[]string{"--expedited=true"},
					s.proposalArgs(
						"expedited tieredrewards params",
						"ensure expedited proposals are wired correctly",
						expeditedDeposit,
						"tieredrewards-expedited-update-params",
					)...,
				)...,
			)
		})
		expeditedSubmitMsg, ok := s.mustGetTxMsg(val, expeditedProposalTx.TxHash).(*v1.MsgSubmitProposal)
		s.Require().True(ok)
		s.Require().True(expeditedSubmitMsg.Expedited)
		s.Require().Equal("expedited tieredrewards params", expeditedSubmitMsg.Title)
		expeditedProposal := s.mustGetProposal(val, s.proposalIDFromTx(expeditedProposalTx))
		s.Require().Equal("expedited tieredrewards params", expeditedProposal.Title)
		expeditedMsg, ok := s.mustUnpackProposalMsg(val, expeditedProposal).(*tieredrewardstypes.MsgUpdateParams)
		s.Require().True(ok)
		s.Require().True(expeditedMsg.Params.TargetBaseRewardsRate.Equal(sdkmath.LegacyMustNewDecFromStr("0.04")))

		addTierPath := s.writeTempJSON("add-tier.json", `{"id":2,"exit_duration":"1s","bonus_apy":"0.50","min_lock_amount":"2000000","close_only":false}`)
		addTierTx := s.mustExecTx(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.AddTierProposalExec(
				val.ClientCtx,
				owner,
				addTierPath,
				s.proposalArgs("add tier", "ensure tier file parsing works", deposit, "tieredrewards-add-tier")...,
			)
		})
		addTierProposal := s.mustGetProposal(val, s.proposalIDFromTx(addTierTx))
		addTierMsg, ok := s.mustUnpackProposalMsg(val, addTierProposal).(*tieredrewardstypes.MsgAddTier)
		s.Require().True(ok)
		s.Require().Equal(uint32(2), addTierMsg.Tier.Id)
		s.Require().Equal(time.Second, addTierMsg.Tier.ExitDuration)
		s.Require().True(addTierMsg.Tier.BonusApy.Equal(sdkmath.LegacyMustNewDecFromStr("0.50")))
		s.Require().True(addTierMsg.Tier.MinLockAmount.Equal(sdkmath.NewInt(2_000_000)))

		updateTierTx := s.mustExecTx(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.UpdateTierProposalExec(
				val.ClientCtx,
				owner,
				`{"id":1,"exit_duration":"1s","bonus_apy":"0.75","min_lock_amount":"3000000","close_only":true}`,
				s.proposalArgs("update tier", "ensure inline tier JSON is parsed", deposit, "tieredrewards-update-tier")...,
			)
		})
		updateTierProposal := s.mustGetProposal(val, s.proposalIDFromTx(updateTierTx))
		updateTierMsg, ok := s.mustUnpackProposalMsg(val, updateTierProposal).(*tieredrewardstypes.MsgUpdateTier)
		s.Require().True(ok)
		s.Require().Equal(tieredrewardstestutil.TestTierID, updateTierMsg.Tier.Id)
		s.Require().True(updateTierMsg.Tier.CloseOnly)
		s.Require().True(updateTierMsg.Tier.BonusApy.Equal(sdkmath.LegacyMustNewDecFromStr("0.75")))
		s.Require().True(updateTierMsg.Tier.MinLockAmount.Equal(sdkmath.NewInt(3_000_000)))

		deleteTierTx := s.mustExecTx(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.DeleteTierProposalExec(
				val.ClientCtx,
				owner,
				"2",
				s.proposalArgs("delete tier", "ensure delete tier proposal wiring works", deposit, "tieredrewards-delete-tier")...,
			)
		})
		deleteTierProposal := s.mustGetProposal(val, s.proposalIDFromTx(deleteTierTx))
		deleteTierMsg, ok := s.mustUnpackProposalMsg(val, deleteTierProposal).(*tieredrewardstypes.MsgDeleteTier)
		s.Require().True(ok)
		s.Require().Equal(uint32(2), deleteTierMsg.Id)
	})

	s.Run("lock position", func() {
		s.mustExecTx(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.LockTierExec(
				val.ClientCtx,
				owner,
				strconv.FormatUint(uint64(tieredrewardstestutil.TestTierID), 10),
				lockAmount.String(),
				validator,
				s.defaultTxArgs()...,
			)
		})

		position := s.mustQueryPosition(val, "0")
		s.Require().Equal(uint64(0), position.Id)
		s.Require().Equal(owner, position.Owner)
		s.Require().Equal(validator, position.Validator)
		s.Require().True(position.Amount.Equal(lockAmount), "amount should equal lock amount for delegated position")
		s.Require().True(position.DelegatedShares.IsPositive(), "delegated position should have positive shares")

		s.mustExecTx(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.AddToTierPositionExec(
				val.ClientCtx,
				owner,
				"0",
				addAmount.String(),
				s.defaultTxArgs()...,
			)
		})

		position = s.mustQueryPosition(val, "0")
		s.Require().True(position.Amount.Equal(lockAmount.Add(addAmount)))
	})

	s.Run("position queries and voting power", func() {
		var positionsByOwnerResp tieredrewardstypes.QueryTierPositionsByOwnerResponse
		s.mustExecQuery(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.QueryTierPositionsByOwnerExec(val.ClientCtx, owner)
		}, &positionsByOwnerResp)
		s.Require().Len(positionsByOwnerResp.Positions, 1)
		s.Require().Equal(uint64(0), positionsByOwnerResp.Positions[0].Id)

		allPositionsResp := s.mustQueryAllPositions(val)
		s.Require().Len(allPositionsResp.Positions, 1)
		s.Require().Equal(uint64(0), allPositionsResp.Positions[0].Id)

		var votingPowerResp tieredrewardstypes.QueryVotingPowerByOwnerResponse
		s.mustExecQuery(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.QueryVotingPowerByOwnerExec(val.ClientCtx, owner)
		}, &votingPowerResp)
		s.Require().True(votingPowerResp.VotingPower.IsPositive())

		var totalVotingPowerResp tieredrewardstypes.QueryTotalDelegatedVotingPowerResponse
		s.mustExecQuery(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.QueryTotalDelegatedVotingPowerExec(val.ClientCtx)
		}, &totalVotingPowerResp)
		s.Require().True(totalVotingPowerResp.VotingPower.Equal(votingPowerResp.VotingPower))
	})

	s.Run("rewards claim and redelegate", func() {
		s.waitBlocks(1)

		var estimateRewardsResp tieredrewardstypes.QueryEstimatePositionRewardsResponse
		s.mustExecQuery(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.QueryEstimatePositionRewardsExec(val.ClientCtx, "0")
		}, &estimateRewardsResp)
		s.Require().True(estimateRewardsResp.BonusRewards.AmountOf(s.cfg.BondDenom).IsPositive())

		s.mustExecTx(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.ClaimTierRewardsExec(
				val.ClientCtx,
				owner,
				"0",
				s.defaultTxArgs()...,
			)
		})

		s.mustExecTx(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.TierRedelegateExec(
				val.ClientCtx,
				owner,
				"0",
				dstValidator,
				s.defaultTxArgs()...,
			)
		})

		position := s.mustQueryPosition(val, "0")
		s.Require().Equal(dstValidator, position.Validator)
	})

	s.Run("exit trigger and clear", func() {
		s.mustExecTx(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.TriggerExitFromTierExec(
				val.ClientCtx,
				owner,
				"0",
				s.defaultTxArgs()...,
			)
		})

		position := s.mustQueryPosition(val, "0")
		s.Require().True(!position.ExitTriggeredAt.IsZero())

		s.mustExecTx(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.ClearPositionExec(
				val.ClientCtx,
				owner,
				"0",
				s.defaultTxArgs()...,
			)
		})

		position = s.mustQueryPosition(val, "0")
		s.Require().False(!position.ExitTriggeredAt.IsZero())
	})

	s.Run("commit delegation and pagination", func() {
		s.mustExecTx(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.CommitDelegationToTierExec(
				val.ClientCtx,
				owner,
				validator,
				commitAmount.String(),
				strconv.FormatUint(uint64(tieredrewardstestutil.TestTierID), 10),
				s.defaultTxArgs()...,
			)
		})

		position := s.mustQueryPosition(val, "1")
		s.Require().Equal(uint64(1), position.Id)
		s.Require().Equal(validator, position.Validator)
		s.Require().True(position.Validator != "")

		firstPageResp := s.mustQueryAllPositions(val, "--limit=1", "--offset=0", "--count-total=true")
		s.Require().Len(firstPageResp.Positions, 1)
		s.Require().NotNil(firstPageResp.Pagination)
		s.Require().Equal(uint64(2), firstPageResp.Pagination.Total)

		secondPageResp := s.mustQueryAllPositions(val, "--limit=1", "--offset=1", "--count-total=true")
		s.Require().Len(secondPageResp.Positions, 1)
		s.Require().NotNil(secondPageResp.Pagination)
		s.Require().Equal(uint64(2), secondPageResp.Pagination.Total)
		s.Require().NotEqual(firstPageResp.Positions[0].Id, secondPageResp.Positions[0].Id)
	})

	s.Run("undelegate and withdraw position", func() {
		s.mustExecTx(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.TriggerExitFromTierExec(
				val.ClientCtx,
				owner,
				"1",
				s.defaultTxArgs()...,
			)
		})

		s.waitBlocks(1)

		s.mustExecTx(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.TierUndelegateExec(
				val.ClientCtx,
				owner,
				"1",
				s.defaultTxArgs()...,
			)
		})

		position := s.mustQueryPosition(val, "1")
		s.Require().False(position.Validator != "")

		s.waitBlocks(1)

		s.mustExecTx(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.WithdrawFromTierExec(
				val.ClientCtx,
				owner,
				"1",
				s.defaultTxArgs()...,
			)
		})
	})

	s.Run("lock with immediate exit and withdraw", func() {
		s.mustExecTx(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.LockTierExec(
				val.ClientCtx,
				owner,
				strconv.FormatUint(uint64(tieredrewardstestutil.TestTierID), 10),
				exitImmediatelyLockAmount.String(),
				validator,
				append([]string{"--trigger-exit-immediately=true"}, s.defaultTxArgs()...)...,
			)
		})

		position := s.mustQueryPosition(val, "2")
		s.Require().True(!position.ExitTriggeredAt.IsZero())

		s.waitBlocks(1)

		s.mustExecTx(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.TierUndelegateExec(
				val.ClientCtx,
				owner,
				"2",
				s.defaultTxArgs()...,
			)
		})

		s.waitBlocks(1)

		s.mustExecTx(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.WithdrawFromTierExec(
				val.ClientCtx,
				owner,
				"2",
				s.defaultTxArgs()...,
			)
		})

		s.assertCLIError(func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.QueryTierPositionExec(val.ClientCtx, "2")
		}, "")
	})

	s.Run("exit tier with delegation", func() {
		exitWithDelegationAmount := sdkmath.NewInt(10_000_000)

		// Lock a new position with immediate exit.
		s.mustExecTx(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.LockTierExec(
				val.ClientCtx,
				owner,
				strconv.FormatUint(uint64(tieredrewardstestutil.TestTierID), 10),
				exitWithDelegationAmount.String(),
				validator,
				append([]string{"--trigger-exit-immediately=true"}, s.defaultTxArgs()...)...,
			)
		})

		position := s.mustQueryPosition(val, "3")
		s.Require().True(!position.ExitTriggeredAt.IsZero())
		s.Require().True(position.Validator != "")

		// Wait for exit duration to elapse.
		s.waitBlocks(1)

		// Exit with delegation (full amount).
		s.mustExecTx(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.ExitTierWithDelegationExec(
				val.ClientCtx,
				owner,
				"3",
				exitWithDelegationAmount.String(),
				s.defaultTxArgs()...,
			)
		})

		// Position should be deleted.
		s.assertCLIError(func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.QueryTierPositionExec(val.ClientCtx, "3")
		}, "")
	})

	s.Run("raw-position", func() {
		var resp tieredrewardstypes.QueryRawTierPositionResponse
		s.mustExecQuery(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.QueryRawTierPositionExec(val.ClientCtx, "0")
		}, &resp)
		s.Require().Equal(owner, resp.Position.Owner)
		s.Require().True(resp.Position.Amount.IsZero(), "raw delegated position amount should be zero")
		s.Require().True(resp.Position.DelegatedShares.IsPositive())
	})

	s.Run("raw-positions-by-owner", func() {
		var resp tieredrewardstypes.QueryRawTierPositionsByOwnerResponse
		s.mustExecQuery(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.QueryRawTierPositionsByOwnerExec(val.ClientCtx, owner)
		}, &resp)
		s.Require().GreaterOrEqual(len(resp.Positions), 1)
	})

	s.Run("raw-all-positions", func() {
		var resp tieredrewardstypes.QueryRawAllTierPositionsResponse
		s.mustExecQuery(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.QueryRawAllTierPositionsExec(val.ClientCtx)
		}, &resp)
		s.Require().GreaterOrEqual(len(resp.Positions), 1)
	})

	s.Run("validator-data", func() {
		var resp tieredrewardstypes.QueryValidatorDataResponse
		s.mustExecQuery(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.QueryValidatorDataExec(val.ClientCtx, validator)
		}, &resp)
	})

	s.Run("position-mappings", func() {
		var resp tieredrewardstypes.QueryPositionMappingsResponse
		s.mustExecQuery(val, func() (sdktestutil.BufferWriter, error) {
			return tieredrewardstestutil.QueryPositionMappingsExec(val.ClientCtx, "0")
		}, &resp)
	})
}

func (s *IntegrationTestSuite) TestTieredRewardsCLIErrors() {
	val := s.network.Validators[0]
	owner := val.Address.String()
	validator := val.ValAddress.String()
	deposit := sdk.NewCoin(s.cfg.BondDenom, sdkmath.OneInt()).String()

	testCases := []struct {
		name         string
		exec         func() (sdktestutil.BufferWriter, error)
		wantContains string
	}{
		{
			name: "query invalid position id",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.QueryTierPositionExec(val.ClientCtx, "not-a-number")
			},
			wantContains: `invalid position-id "not-a-number"`,
		},
		{
			name: "query invalid owner",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.QueryTierPositionsByOwnerExec(val.ClientCtx, "not-an-address")
			},
		},
		{
			name: "query invalid voter",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.QueryVotingPowerByOwnerExec(val.ClientCtx, "not-an-address")
			},
		},
		{
			name: "query missing position",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.QueryTierPositionExec(val.ClientCtx, "999")
			},
			wantContains: "position not found",
		},
		{
			name: "proposal invalid params json",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.UpdateParamsProposalExec(
					val.ClientCtx,
					owner,
					`{"target_base_rewards_rate":`,
					s.proposalArgs("bad params", "invalid json should fail", deposit, "")...,
				)
			},
		},
		{
			name: "proposal missing required flags",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.AddTierProposalExec(
					val.ClientCtx,
					owner,
					`{"id":0,"exit_duration":"1s","bonus_apy":"0.1","min_lock_amount":"1"}`,
					append([]string{}, s.defaultTxArgs()...)...,
				)
			},
			wantContains: "required flag(s)",
		},
		{
			name: "proposal invalid delete id",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.DeleteTierProposalExec(
					val.ClientCtx,
					owner,
					"abc",
					s.proposalArgs("bad delete", "invalid id should fail", deposit, "")...,
				)
			},
			wantContains: `invalid id "abc"`,
		},
		{
			name: "lock tier invalid validator",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.LockTierExec(
					val.ClientCtx,
					owner,
					strconv.FormatUint(uint64(tieredrewardstestutil.TestTierID), 10),
					"1000000",
					"invalid-validator",
					s.defaultTxArgs()...,
				)
			},
		},
		{
			name: "commit delegation invalid amount",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.CommitDelegationToTierExec(
					val.ClientCtx,
					owner,
					validator,
					"not-an-amount",
					strconv.FormatUint(uint64(tieredrewardstestutil.TestTierID), 10),
					s.defaultTxArgs()...,
				)
			},
			wantContains: `invalid amount "not-an-amount"`,
		},
		{
			name: "tier undelegate invalid position id",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.TierUndelegateExec(
					val.ClientCtx,
					owner,
					"not-a-number",
					s.defaultTxArgs()...,
				)
			},
			wantContains: `invalid position-id "not-a-number"`,
		},
		{
			name: "tier redelegate invalid position id",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.TierRedelegateExec(
					val.ClientCtx,
					owner,
					"not-a-number",
					validator,
					s.defaultTxArgs()...,
				)
			},
			wantContains: `invalid position-id "not-a-number"`,
		},
		{
			name: "add to position invalid amount",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.AddToTierPositionExec(
					val.ClientCtx,
					owner,
					"0",
					"not-an-amount",
					s.defaultTxArgs()...,
				)
			},
			wantContains: `invalid amount "not-an-amount"`,
		},
		{
			name: "trigger exit invalid position id",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.TriggerExitFromTierExec(
					val.ClientCtx,
					owner,
					"not-a-number",
					s.defaultTxArgs()...,
				)
			},
			wantContains: `invalid position-id "not-a-number"`,
		},
		{
			name: "clear position invalid position id",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.ClearPositionExec(
					val.ClientCtx,
					owner,
					"not-a-number",
					s.defaultTxArgs()...,
				)
			},
			wantContains: `invalid position-id "not-a-number"`,
		},
		{
			name: "claim rewards invalid position id",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.ClaimTierRewardsExec(
					val.ClientCtx,
					owner,
					"not-a-number",
					s.defaultTxArgs()...,
				)
			},
			wantContains: `invalid position-id "not-a-number"`,
		},
		{
			name: "withdraw invalid position id",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.WithdrawFromTierExec(
					val.ClientCtx,
					owner,
					"not-a-number",
					s.defaultTxArgs()...,
				)
			},
			wantContains: `invalid position-id "not-a-number"`,
		},
		{
			name: "lock tier invalid tier id",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.LockTierExec(
					val.ClientCtx,
					owner,
					"abc",
					"1000000",
					validator,
					s.defaultTxArgs()...,
				)
			},
			wantContains: `invalid id "abc"`,
		},
		{
			name: "lock tier invalid amount",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.LockTierExec(
					val.ClientCtx,
					owner,
					strconv.FormatUint(uint64(tieredrewardstestutil.TestTierID), 10),
					"not-an-amount",
					validator,
					s.defaultTxArgs()...,
				)
			},
			wantContains: `invalid amount "not-an-amount"`,
		},
		{
			name: "commit delegation invalid tier id",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.CommitDelegationToTierExec(
					val.ClientCtx,
					owner,
					validator,
					"1000000",
					"abc",
					s.defaultTxArgs()...,
				)
			},
			wantContains: `invalid id "abc"`,
		},
		{
			name: "add to position invalid position id",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.AddToTierPositionExec(
					val.ClientCtx,
					owner,
					"not-a-number",
					"1000000",
					s.defaultTxArgs()...,
				)
			},
			wantContains: `invalid position-id "not-a-number"`,
		},
		{
			name: "estimate rewards invalid position id",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.QueryEstimatePositionRewardsExec(val.ClientCtx, "not-a-number")
			},
			wantContains: `invalid position-id "not-a-number"`,
		},
		{
			name: "proposal with file-like path that does not exist",
			exec: func() (sdktestutil.BufferWriter, error) {
				return tieredrewardstestutil.UpdateParamsProposalExec(
					val.ClientCtx,
					owner,
					"/tmp/nonexistent-file.json",
					s.proposalArgs("bad path", "file should not exist", deposit, "")...,
				)
			},
			wantContains: "file not found",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.assertCLIError(tc.exec, tc.wantContains)
		})
	}
}
