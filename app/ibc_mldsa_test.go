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

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const counterpartyChainID = "counterparty-1"

func (suite *AppTestSuite) mlDsaKey() crypto.PrivKey {
	key, err := mldsa65.GenPrivKey()
	suite.Require().NoError(err)
	return key
}

func (suite *AppTestSuite) signedHeader(
	valSet *cmttypes.ValidatorSet, signers map[string]cmttypes.PrivValidator, height int64, blockTime time.Time,
) *cmtproto.SignedHeader {
	unusedHash := tmhash.Sum([]byte{0x00})
	header := cmttypes.Header{
		Version:            cmtversion.Consensus{Block: version.BlockProtocol, App: 2},
		ChainID:            counterpartyChainID,
		Height:             height,
		Time:               blockTime,
		LastBlockID:        cmttypes.BlockID{Hash: make([]byte, tmhash.Size), PartSetHeader: cmttypes.PartSetHeader{Total: 1, Hash: make([]byte, tmhash.Size)}},
		LastCommitHash:     unusedHash,
		DataHash:           unusedHash,
		ValidatorsHash:     valSet.Hash(),
		NextValidatorsHash: valSet.Hash(),
		ConsensusHash:      unusedHash,
		AppHash:            unusedHash,
		LastResultsHash:    unusedHash,
		EvidenceHash:       unusedHash,
		ProposerAddress:    valSet.Proposer.Address,
	}

	// MakeExtCommit expects signers in validator set order.
	privVals := make([]cmttypes.PrivValidator, len(valSet.Validators))
	for i, val := range valSet.Validators {
		privVals[i] = signers[val.Address.String()]
	}
	blockID := cmttypes.BlockID{
		Hash:          header.Hash(),
		PartSetHeader: cmttypes.PartSetHeader{Total: 3, Hash: unusedHash},
	}
	voteSet := cmttypes.NewVoteSet(counterpartyChainID, height, 1, cmtproto.PrecommitType, valSet)
	extCommit, err := cmttypes.MakeExtCommit(blockID, height, 1, voteSet, privVals, blockTime, false)
	suite.Require().NoError(err)

	return &cmtproto.SignedHeader{Header: header.ToProto(), Commit: extCommit.ToCommit().ToProto()}
}

// Relays a counterparty header with ML-DSA-65 validators through the MsgCreateClient and
// MsgUpdateClient handlers, as a relayer tx would reach them.
func (suite *AppTestSuite) TestUpdateClientWithMlDsaValidators() {
	testCases := []struct {
		name     string
		keys     func() []crypto.PrivKey
		malleate func(*ibctm.Header)
		expErr   string
	}{
		{
			name:     "ML-DSA-65 validators",
			keys:     func() []crypto.PrivKey { return []crypto.PrivKey{suite.mlDsaKey(), suite.mlDsaKey()} },
			malleate: func(*ibctm.Header) {},
		},
		{
			name:     "ed25519 and ML-DSA-65 validators",
			keys:     func() []crypto.PrivKey { return []crypto.PrivKey{ed25519.GenPrivKey(), suite.mlDsaKey()} },
			malleate: func(*ibctm.Header) {},
		},
		{
			name: "tampered ML-DSA-65 signature",
			keys: func() []crypto.PrivKey { return []crypto.PrivKey{suite.mlDsaKey()} },
			malleate: func(h *ibctm.Header) {
				h.SignedHeader.Commit.Signatures[0].Signature[0] ^= 0xff
			},
			expErr: "wrong signature",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			relayer := sdk.AccAddress(ed25519.GenPrivKey().PubKey().Address()).String()

			keys := tc.keys()
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
				SignedHeader:      suite.signedHeader(valSet, signers, 2, trustedTime.Add(time.Second)),
				ValidatorSet:      valSetProto,
				TrustedHeight:     trustedHeight,
				TrustedValidators: valSetProto,
			}
			tc.malleate(header)

			updateMsg, err := clienttypes.NewMsgUpdateClient(created.ClientId, header, relayer)
			suite.Require().NoError(err)
			suite.Require().NoError(updateMsg.ValidateBasic())
			_, err = suite.app.MsgServiceRouter().Handler(updateMsg)(suite.ctx, updateMsg)
			if tc.expErr != "" {
				suite.Require().ErrorContains(err, tc.expErr)
				return
			}
			suite.Require().NoError(err)

			_, found := suite.app.IBCKeeper.ClientKeeper.GetClientConsensusState(suite.ctx, created.ClientId, clienttypes.NewHeight(1, 2))
			suite.Require().True(found)
		})
	}
}
