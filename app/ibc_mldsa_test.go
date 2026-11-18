package app_test

import (
	"time"

	"github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/crypto/mldsa65"
	"github.com/cometbft/cometbft/crypto/tmhash"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmtversion "github.com/cometbft/cometbft/proto/tendermint/version"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cometbft/cometbft/version"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	commitmenttypes "github.com/cosmos/ibc-go/v10/modules/core/23-commitment/types"
	ibctm "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const counterpartyChainID = "counterparty-1"

func (suite *AppTestSuite) mlDSAKey() crypto.PrivKey {
	suite.T().Helper()
	key, err := mldsa65.GenPrivKey()
	suite.Require().NoError(err)
	return key
}

func (suite *AppTestSuite) signedHeader(
	valSet *cmttypes.ValidatorSet, signers map[string]cmttypes.PrivValidator, height int64, blockTime time.Time,
) *cmtproto.SignedHeader {
	suite.T().Helper()
	unusedHash := tmhash.Sum([]byte{0x00})
	signedHeader, err := ibctesting.CommitHeader(cmttypes.Header{
		Version:            cmtversion.Consensus{Block: version.BlockProtocol, App: 2},
		ChainID:            counterpartyChainID,
		Height:             height,
		Time:               blockTime,
		LastBlockID:        ibctesting.MakeBlockID(make([]byte, tmhash.Size), 1, make([]byte, tmhash.Size)),
		LastCommitHash:     unusedHash,
		DataHash:           unusedHash,
		ValidatorsHash:     valSet.Hash(),
		NextValidatorsHash: valSet.Hash(),
		ConsensusHash:      unusedHash,
		AppHash:            unusedHash,
		LastResultsHash:    unusedHash,
		EvidenceHash:       unusedHash,
		ProposerAddress:    valSet.Proposer.Address,
	}, valSet, signers)
	suite.Require().NoError(err)
	return signedHeader
}

func (suite *AppTestSuite) TestUpdateClientWithMLDSAValidators() {
	tamper := func(h *ibctm.Header) { h.SignedHeader.Commit.Signatures[0].Signature[0] ^= 0xff }
	testCases := []struct {
		name     string
		keys     []crypto.PrivKey
		height   int64
		malleate func(*ibctm.Header)
		expErr   string
	}{
		{
			name:   "ML-DSA-65 validators",
			keys:   []crypto.PrivKey{suite.mlDSAKey(), suite.mlDSAKey()},
			height: 2,
		},
		{
			name:   "ML-DSA-65 validators, non-adjacent height",
			keys:   []crypto.PrivKey{suite.mlDSAKey(), suite.mlDSAKey()},
			height: 5,
		},
		{
			name:   "ed25519 and ML-DSA-65 validators",
			keys:   []crypto.PrivKey{ed25519.GenPrivKey(), suite.mlDSAKey()},
			height: 2,
		},
		{
			name:     "tampered ML-DSA-65 signature",
			keys:     []crypto.PrivKey{suite.mlDSAKey()},
			height:   2,
			malleate: tamper,
			expErr:   "wrong signature",
		},
		{
			name:     "tampered ML-DSA-65 signature, non-adjacent height",
			keys:     []crypto.PrivKey{suite.mlDSAKey()},
			height:   5,
			malleate: tamper,
			expErr:   "wrong signature",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			relayer := sdk.AccAddress(ed25519.GenPrivKey().PubKey().Address()).String()

			keys := tc.keys
			vals := make([]*cmttypes.Validator, len(keys))
			signers := make(map[string]cmttypes.PrivValidator, len(keys))
			for i, key := range keys {
				vals[i] = cmttypes.NewValidator(key.PubKey(), 10)
				signers[key.PubKey().Address().String()] = cmttypes.NewMockPVWithParams(key, false, false)
			}
			valSet := cmttypes.NewValidatorSet(vals)

			trustedTime := suite.ctx.BlockTime().Add(-time.Minute)
			trustedHeight := clienttypes.NewHeight(1, 1)
			clientState := ibctm.NewClientState(
				counterpartyChainID, ibctm.DefaultTrustLevel, time.Hour, 2*time.Hour, 10*time.Second,
				trustedHeight, commitmenttypes.GetSDKSpecs(), []string{"upgrade", "upgradedIBCState"},
			)
			consState := ibctm.NewConsensusState(trustedTime, commitmenttypes.NewMerkleRoot(tmhash.Sum([]byte("app"))), valSet.Hash())
			createMsg, err := clienttypes.NewMsgCreateClient(clientState, consState, relayer)
			suite.Require().NoError(err)
			createRes, err := suite.app.MsgServiceRouter().Handler(createMsg)(suite.ctx, createMsg)
			suite.Require().NoError(err)
			var created clienttypes.MsgCreateClientResponse
			suite.Require().NoError(suite.app.AppCodec().Unmarshal(createRes.MsgResponses[0].Value, &created))

			valSetProto, err := valSet.ToProto()
			suite.Require().NoError(err)
			header := &ibctm.Header{
				SignedHeader:      suite.signedHeader(valSet, signers, tc.height, trustedTime.Add(time.Second)),
				ValidatorSet:      valSetProto,
				TrustedHeight:     trustedHeight,
				TrustedValidators: valSetProto,
			}
			if tc.malleate != nil {
				tc.malleate(header)
			}

			updateMsg, err := clienttypes.NewMsgUpdateClient(created.ClientId, header, relayer)
			suite.Require().NoError(err)
			suite.Require().NoError(updateMsg.ValidateBasic())
			_, err = suite.app.MsgServiceRouter().Handler(updateMsg)(suite.ctx, updateMsg)
			if tc.expErr != "" {
				suite.Require().ErrorContains(err, tc.expErr)
				return
			}
			suite.Require().NoError(err)

			_, found := suite.app.IBCKeeper.ClientKeeper.GetClientConsensusState(suite.ctx, created.ClientId, header.GetHeight())
			suite.Require().True(found)
		})
	}
}
