package app_test

import (
	"os"
	"path/filepath"
	"time"

	"github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/crypto/mldsa65"
	"github.com/cometbft/cometbft/crypto/tmhash"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmtversion "github.com/cometbft/cometbft/proto/tendermint/version"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cometbft/cometbft/version"
	"github.com/cosmos/gogoproto/proto"
	clienttypes "github.com/lfdt-ibc/ibc-go/v11/modules/core/02-client/types"
	commitmenttypes "github.com/lfdt-ibc/ibc-go/v11/modules/core/23-commitment/types"
	"github.com/lfdt-ibc/ibc-go/v11/modules/core/exported"
	ibctm "github.com/lfdt-ibc/ibc-go/v11/modules/light-clients/07-tendermint"
	ibctesting "github.com/lfdt-ibc/ibc-go/v11/testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	counterpartyChainID = "counterparty-1"

	// mlDSAFixtureDir holds the client, consensus state and header the upgrade integration test submits.
	mlDSAFixtureDir = "../integration_tests/data/mldsa65"
	// Set to regenerate the fixtures: UPDATE_MLDSA_FIXTURES=1 go test ./app -run TestAppTestSuite/TestMLDSAClientFixtures
	updateMLDSAFixturesEnv = "UPDATE_MLDSA_FIXTURES"
	// The fixtures are signed once, so their trusting period must outlive any chain that replays them.
	fixtureTrustingPeriod = 100 * 365 * 24 * time.Hour
)

var fixtureTrustedTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

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

// counterpartyClient returns a client trusted at height 1-1 and a header at height, signed by keys one second after trustedTime.
func (suite *AppTestSuite) counterpartyClient(
	keys []crypto.PrivKey, height int64, trustedTime time.Time, trustingPeriod time.Duration,
) (*ibctm.ClientState, *ibctm.ConsensusState, *ibctm.Header) {
	suite.T().Helper()
	vals := make([]*cmttypes.Validator, len(keys))
	signers := make(map[string]cmttypes.PrivValidator, len(keys))
	for i, key := range keys {
		vals[i] = cmttypes.NewValidator(key.PubKey(), 10)
		signers[key.PubKey().Address().String()] = cmttypes.NewMockPVWithParams(key, false, false)
	}
	valSet := cmttypes.NewValidatorSet(vals)

	trustedHeight := clienttypes.NewHeight(1, 1)
	clientState := ibctm.NewClientState(
		counterpartyChainID, ibctm.DefaultTrustLevel, trustingPeriod, 2*trustingPeriod, 10*time.Second,
		trustedHeight, commitmenttypes.GetSDKSpecs(), []string{"upgrade", "upgradedIBCState"},
	)
	consState := ibctm.NewConsensusState(trustedTime, commitmenttypes.NewMerkleRoot(tmhash.Sum([]byte("app"))), valSet.Hash())

	valSetProto, err := valSet.ToProto()
	suite.Require().NoError(err)
	header := &ibctm.Header{
		SignedHeader:      suite.signedHeader(valSet, signers, height, trustedTime.Add(time.Second)),
		ValidatorSet:      valSetProto,
		TrustedHeight:     trustedHeight,
		TrustedValidators: valSetProto,
	}
	return clientState, consState, header
}

// createAndUpdateClient routes MsgCreateClient then MsgUpdateClient and returns the client id and the update error.
func (suite *AppTestSuite) createAndUpdateClient(
	clientState exported.ClientState, consState exported.ConsensusState, header exported.ClientMessage,
) (string, error) {
	suite.T().Helper()
	relayer := sdk.AccAddress(ed25519.GenPrivKey().PubKey().Address()).String()

	createMsg, err := clienttypes.NewMsgCreateClient(clientState, consState, relayer)
	suite.Require().NoError(err)
	createRes, err := suite.app.MsgServiceRouter().Handler(createMsg)(suite.ctx, createMsg)
	suite.Require().NoError(err)
	var created clienttypes.MsgCreateClientResponse
	suite.Require().NoError(suite.app.AppCodec().Unmarshal(createRes.MsgResponses[0].Value, &created))

	updateMsg, err := clienttypes.NewMsgUpdateClient(created.ClientId, header, relayer)
	suite.Require().NoError(err)
	suite.Require().NoError(updateMsg.ValidateBasic())
	_, err = suite.app.MsgServiceRouter().Handler(updateMsg)(suite.ctx, updateMsg)
	return created.ClientId, err
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
			clientState, consState, header := suite.counterpartyClient(
				tc.keys, tc.height, suite.ctx.BlockTime().Add(-time.Minute), time.Hour,
			)
			if tc.malleate != nil {
				tc.malleate(header)
			}

			clientID, err := suite.createAndUpdateClient(clientState, consState, header)
			if tc.expErr != "" {
				suite.Require().ErrorContains(err, tc.expErr)
				return
			}
			suite.Require().NoError(err)

			_, found := suite.app.IBCKeeper.ClientKeeper.GetClientConsensusState(suite.ctx, clientID, header.GetHeight())
			suite.Require().True(found)
		})
	}
}

func (suite *AppTestSuite) TestMLDSAClientFixtures() {
	cdc := suite.app.AppCodec()
	if os.Getenv(updateMLDSAFixturesEnv) != "" {
		clientState, consState, header := suite.counterpartyClient(
			[]crypto.PrivKey{suite.mlDSAKey(), suite.mlDSAKey()}, 2, fixtureTrustedTime, fixtureTrustingPeriod,
		)
		files := map[string]proto.Message{
			"client_state.json":    clientState,
			"consensus_state.json": consState,
			"header.json":          header,
		}
		suite.Require().NoError(os.MkdirAll(mlDSAFixtureDir, 0o755))
		for name, msg := range files {
			bz, err := cdc.MarshalInterfaceJSON(msg)
			suite.Require().NoError(err)
			suite.Require().NoError(os.WriteFile(filepath.Join(mlDSAFixtureDir, name), append(bz, '\n'), 0o600))
		}
	}

	// Decode the way `chain-maind tx ibc client create/update` does, so the fixtures stay valid for the CLI.
	read := func(name string, ptr any) {
		bz, err := os.ReadFile(filepath.Join(mlDSAFixtureDir, name))
		suite.Require().NoError(err)
		suite.Require().NoError(cdc.UnmarshalInterfaceJSON(bz, ptr))
	}
	var (
		clientState exported.ClientState
		consState   exported.ConsensusState
		header      exported.ClientMessage
	)
	read("client_state.json", &clientState)
	read("consensus_state.json", &consState)
	read("header.json", &header)

	tmHeader, ok := header.(*ibctm.Header)
	suite.Require().True(ok)
	for _, val := range tmHeader.ValidatorSet.Validators {
		suite.Require().NotNil(val.PubKey.GetMldsa65(), "fixture validators must use ML-DSA-65 keys")
	}

	clientID, err := suite.createAndUpdateClient(clientState, consState, header)
	suite.Require().NoError(err)
	_, found := suite.app.IBCKeeper.ClientKeeper.GetClientConsensusState(suite.ctx, clientID, tmHeader.GetHeight())
	suite.Require().True(found)
}
