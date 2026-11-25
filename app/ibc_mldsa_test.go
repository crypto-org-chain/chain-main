package app_test

import (
	"encoding/json"
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
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/gogoproto/proto"
	"github.com/crypto-org-chain/chain-main/v8/app"
	"github.com/crypto-org-chain/chain-main/v8/testutil"
	transfertypes "github.com/lfdt-ibc/ibc-go/v11/modules/apps/transfer/types"
	clienttypes "github.com/lfdt-ibc/ibc-go/v11/modules/core/02-client/types"
	channeltypes "github.com/lfdt-ibc/ibc-go/v11/modules/core/04-channel/types"
	commitmenttypes "github.com/lfdt-ibc/ibc-go/v11/modules/core/23-commitment/types"
	"github.com/lfdt-ibc/ibc-go/v11/modules/core/exported"
	ibckeeper "github.com/lfdt-ibc/ibc-go/v11/modules/core/keeper"
	ibctm "github.com/lfdt-ibc/ibc-go/v11/modules/light-clients/07-tendermint"
	ibctesting "github.com/lfdt-ibc/ibc-go/v11/testing"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	counterpartyChainID = "counterparty-1"

	errWrongSignature = "wrong signature"

	// mlDSAFixtureDir holds the client, consensus state and header the upgrade integration test submits.
	mlDSAFixtureDir = "../integration_tests/data/mldsa65"
	// Set to regenerate the fixtures: UPDATE_MLDSA_FIXTURES=1 go test ./app -run TestAppTestSuite/TestMLDSAClientFixtures
	updateMLDSAFixturesEnv = "UPDATE_MLDSA_FIXTURES"
	// The fixtures are signed once, so their trusting period must outlive any chain that replays them.
	fixtureTrustingPeriod = 100 * 365 * 24 * time.Hour
)

var (
	fixtureTrustedTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	counterpartyTrustedHeight = clienttypes.NewHeight(1, 1)

	relayerAddress = sdk.AccAddress(ed25519.GenPrivKey().PubKey().Address()).String()
)

// ibcTestingApp adds the accessors ibctesting needs to drive a ChainApp as one side of a path.
type ibcTestingApp struct{ *app.ChainApp }

func (a ibcTestingApp) GetBaseApp() *baseapp.BaseApp    { return a.BaseApp }
func (a ibcTestingApp) GetIBCKeeper() *ibckeeper.Keeper { return a.IBCKeeper }
func (a ibcTestingApp) GetTxConfig() client.TxConfig    { return a.TxConfig() }

func (suite *AppTestSuite) mlDSAKey() crypto.PrivKey {
	suite.T().Helper()
	key, err := mldsa65.GenPrivKey()
	suite.Require().NoError(err)
	return key
}

func (suite *AppTestSuite) mlDSAKeys(n int) []crypto.PrivKey {
	suite.T().Helper()
	keys := make([]crypto.PrivKey, n)
	for i := range keys {
		keys[i] = suite.mlDSAKey()
	}
	return keys
}

func validatorSet(keys []crypto.PrivKey) (*cmttypes.ValidatorSet, map[string]cmttypes.PrivValidator) {
	vals := make([]*cmttypes.Validator, len(keys))
	signers := make(map[string]cmttypes.PrivValidator, len(keys))
	for i, key := range keys {
		vals[i] = cmttypes.NewValidator(key.PubKey(), 10)
		signers[key.PubKey().Address().String()] = cmttypes.NewMockPVWithParams(key, false, false)
	}
	return cmttypes.NewValidatorSet(vals), signers
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

// counterpartyClient returns a client that trusts trustedKeys as the validators after counterpartyTrustedHeight.
func (suite *AppTestSuite) counterpartyClient(
	trustedKeys []crypto.PrivKey, trustedTime time.Time, trustingPeriod time.Duration,
) (*ibctm.ClientState, *ibctm.ConsensusState) {
	suite.T().Helper()
	trustedVals, _ := validatorSet(trustedKeys)
	clientState := ibctm.NewClientState(
		counterpartyChainID, ibctm.DefaultTrustLevel, trustingPeriod, 2*trustingPeriod, 10*time.Second,
		counterpartyTrustedHeight, commitmenttypes.GetSDKSpecs(), []string{"upgrade", "upgradedIBCState"},
	)
	consState := ibctm.NewConsensusState(trustedTime, commitmenttypes.NewMerkleRoot(tmhash.Sum([]byte("app"))), trustedVals.Hash())
	return clientState, consState
}

// counterpartyHeader returns a header signed by keys for a client from counterpartyClient(trustedKeys, ...).
func (suite *AppTestSuite) counterpartyHeader(
	trustedKeys, keys []crypto.PrivKey, height int64, blockTime time.Time,
) *ibctm.Header {
	suite.T().Helper()
	trustedVals, _ := validatorSet(trustedKeys)
	trustedValsProto, err := trustedVals.ToProto()
	suite.Require().NoError(err)
	valSet, signers := validatorSet(keys)
	valSetProto, err := valSet.ToProto()
	suite.Require().NoError(err)
	return &ibctm.Header{
		SignedHeader:      suite.signedHeader(valSet, signers, height, blockTime),
		ValidatorSet:      valSetProto,
		TrustedHeight:     counterpartyTrustedHeight,
		TrustedValidators: trustedValsProto,
	}
}

func (suite *AppTestSuite) createClient(clientState exported.ClientState, consState exported.ConsensusState) string {
	suite.T().Helper()
	msg, err := clienttypes.NewMsgCreateClient(clientState, consState, relayerAddress)
	suite.Require().NoError(err)
	res, err := suite.app.MsgServiceRouter().Handler(msg)(suite.ctx, msg)
	suite.Require().NoError(err)
	var created clienttypes.MsgCreateClientResponse
	suite.Require().NoError(suite.app.AppCodec().Unmarshal(res.MsgResponses[0].Value, &created))
	return created.ClientId
}

func (suite *AppTestSuite) updateClient(clientID string, clientMsg exported.ClientMessage) error {
	suite.T().Helper()
	msg, err := clienttypes.NewMsgUpdateClient(clientID, clientMsg, relayerAddress)
	suite.Require().NoError(err)
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	_, err = suite.app.MsgServiceRouter().Handler(msg)(suite.ctx, msg)
	return err
}

func tamperSignature(h *ibctm.Header) { h.Commit.Signatures[0].Signature[0] ^= 0xff }

func (suite *AppTestSuite) TestUpdateClientWithMLDSAValidators() {
	edKeys := []crypto.PrivKey{ed25519.GenPrivKey(), ed25519.GenPrivKey()}
	testCases := []struct {
		name string
		// trustedKeys defaults to keys.
		trustedKeys []crypto.PrivKey
		keys        []crypto.PrivKey
		height      int64
		malleate    func(*ibctm.Header)
		expErr      string
	}{
		{
			name:   "ML-DSA-65 validators",
			keys:   suite.mlDSAKeys(2),
			height: 2,
		},
		{
			name:   "ML-DSA-65 validators, non-adjacent height",
			keys:   suite.mlDSAKeys(2),
			height: 5,
		},
		{
			name:   "ed25519 and ML-DSA-65 validators",
			keys:   []crypto.PrivKey{ed25519.GenPrivKey(), suite.mlDSAKey()},
			height: 2,
		},
		{
			name:        "ML-DSA-65 validators join while a trusted ed25519 validator still signs",
			trustedKeys: edKeys,
			keys:        append(suite.mlDSAKeys(2), edKeys[0]),
			height:      5,
		},
		{
			name:        "ML-DSA-65 validators replace every trusted validator",
			trustedKeys: edKeys,
			keys:        suite.mlDSAKeys(2),
			height:      5,
			expErr:      "cant trust new val set",
		},
		{
			name:     "tampered ML-DSA-65 signature",
			keys:     suite.mlDSAKeys(1),
			height:   2,
			malleate: tamperSignature,
			expErr:   errWrongSignature,
		},
		{
			name:     "tampered ML-DSA-65 signature, non-adjacent height",
			keys:     suite.mlDSAKeys(1),
			height:   5,
			malleate: tamperSignature,
			expErr:   errWrongSignature,
		},
		{
			name:   "ed25519 signature for an ML-DSA-65 validator",
			keys:   suite.mlDSAKeys(1),
			height: 2,
			malleate: func(h *ibctm.Header) {
				sig, err := ed25519.GenPrivKey().Sign([]byte("vote"))
				suite.Require().NoError(err)
				h.Commit.Signatures[0].Signature = sig
			},
			expErr: errWrongSignature,
		},
		{
			name:   "ML-DSA-65 signatures below two thirds of the voting power",
			keys:   suite.mlDSAKeys(3),
			height: 2,
			malleate: func(h *ibctm.Header) {
				absent := cmttypes.NewCommitSigAbsent()
				h.Commit.Signatures[0] = *absent.ToProto()
			},
			expErr: "insufficient voting power",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			trustedKeys := tc.trustedKeys
			if trustedKeys == nil {
				trustedKeys = tc.keys
			}
			trustedTime := suite.ctx.BlockTime().Add(-time.Minute)
			clientState, consState := suite.counterpartyClient(trustedKeys, trustedTime, time.Hour)
			header := suite.counterpartyHeader(trustedKeys, tc.keys, tc.height, trustedTime.Add(time.Second))
			if tc.malleate != nil {
				tc.malleate(header)
			}

			clientID := suite.createClient(clientState, consState)
			err := suite.updateClient(clientID, header)
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

func (suite *AppTestSuite) TestMisbehaviorWithMLDSAValidators() {
	testCases := []struct {
		name string
		// conflictingKeys sign the second header and default to the trusted validators.
		conflictingKeys []crypto.PrivKey
		malleate        func(conflicting *ibctm.Header)
		expErr          string
		expStatus       exported.Status
	}{
		{
			name:      "conflicting ML-DSA-65 headers freeze the client",
			expStatus: exported.Frozen,
		},
		{
			name:      "forged ML-DSA-65 signature leaves the client active",
			malleate:  tamperSignature,
			expErr:    "validator set did not commit to header",
			expStatus: exported.Active,
		},
		{
			name:            "conflicting header from untrusted ML-DSA-65 validators leaves the client active",
			conflictingKeys: suite.mlDSAKeys(2),
			expErr:          "too much change from trusted validator set",
			expStatus:       exported.Active,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			keys := suite.mlDSAKeys(2)
			trustedTime := suite.ctx.BlockTime().Add(-time.Minute)
			clientState, consState := suite.counterpartyClient(keys, trustedTime, time.Hour)
			conflictingKeys := tc.conflictingKeys
			if conflictingKeys == nil {
				conflictingKeys = keys
			}
			clientID := suite.createClient(clientState, consState)
			evidence := &ibctm.Misbehaviour{ //nolint:misspell // ibc-go spells the type this way
				ClientId: clientID,
				Header1:  suite.counterpartyHeader(keys, keys, 2, trustedTime.Add(time.Second)),
				Header2:  suite.counterpartyHeader(keys, conflictingKeys, 2, trustedTime.Add(2*time.Second)),
			}
			if tc.malleate != nil {
				tc.malleate(evidence.Header2)
			}

			err := suite.updateClient(clientID, evidence)
			if tc.expErr != "" {
				suite.Require().ErrorContains(err, tc.expErr)
			} else {
				suite.Require().NoError(err)
			}
			suite.Require().Equal(tc.expStatus, suite.app.IBCKeeper.ClientKeeper.GetClientStatus(suite.ctx, clientID))
		})
	}
}

func (suite *AppTestSuite) TestTransferOverMLDSACounterparty() {
	testCases := []struct {
		name string
		keys []crypto.PrivKey
		// rotatedKeys replace the counterparty's validators once the channel is open.
		rotatedKeys []crypto.PrivKey
	}{
		{
			name: "ML-DSA-65 validators",
			keys: suite.mlDSAKeys(3),
		},
		{
			name: "ed25519 and ML-DSA-65 validators",
			keys: []crypto.PrivKey{ed25519.GenPrivKey(), suite.mlDSAKey(), suite.mlDSAKey()},
		},
		{
			name:        "ed25519 validators rotate to ML-DSA-65 on an open channel",
			keys:        []crypto.PrivKey{ed25519.GenPrivKey(), ed25519.GenPrivKey(), ed25519.GenPrivKey()},
			rotatedKeys: suite.mlDSAKeys(3),
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			t := suite.T()
			coord := &ibctesting.Coordinator{T: t, CurrentTime: fixtureTrustedTime, Chains: make(map[string]*ibctesting.TestChain)}

			var chainApp *app.ChainApp
			chain := ibctesting.NewCustomAppTestChain(t, coord, testutil.ChainID, func() (ibctesting.TestingApp, map[string]json.RawMessage) {
				chainApp = app.New(log.NewNopLogger(), dbm.NewMemDB(), true, simtestutil.AppOptionsMap{flags.FlagHome: t.TempDir()})
				return ibcTestingApp{chainApp}, chainApp.DefaultGenesis()
			})
			valSet, signers := validatorSet(tc.keys)
			counterparty := ibctesting.NewTestChainWithValSet(t, coord, counterpartyChainID, valSet, signers)
			coord.Chains[chain.ChainID] = chain
			coord.Chains[counterparty.ChainID] = counterparty

			path := ibctesting.NewTransferPath(chain, counterparty)
			path.Setup()

			if tc.rotatedKeys != nil {
				var rotatedSigners map[string]cmttypes.PrivValidator
				valSet, rotatedSigners = validatorSet(tc.rotatedKeys)
				for addr, signer := range rotatedSigners {
					counterparty.Signers[addr] = signer
				}
				counterparty.NextVals = valSet
				counterparty.ProposedHeader.NextValidatorsHash = valSet.Hash()
				// The client must see the block that names the new validators: it trusts no signer of any later block.
				suite.Require().NoError(path.EndpointA.UpdateClient())
			}

			user := chain.SenderAccount.GetAddress()
			counterpartyUser := counterparty.SenderAccount.GetAddress()
			transfer := func(from *ibctesting.Endpoint, sender, receiver sdk.AccAddress, timeoutHeight clienttypes.Height) channeltypes.Packet {
				msg := transfertypes.NewMsgTransfer(
					from.ChannelConfig.PortID, from.ChannelID, ibctesting.TestCoin, sender.String(), receiver.String(), timeoutHeight, 0, "",
				)
				res, err := from.Chain.SendMsgs(msg)
				suite.Require().NoError(err)
				packet, err := ibctesting.ParseV1PacketFromEvents(res.Events)
				suite.Require().NoError(err)
				return packet
			}
			balance := func(addr sdk.AccAddress, denom string) sdk.Coin {
				return chainApp.BankKeeper.GetBalance(chain.GetContext(), addr, denom)
			}

			// Receiving proves the packet commitment against a counterparty header.
			packet := transfer(path.EndpointB, counterpartyUser, user, counterparty.GetTimeoutHeight())
			suite.Require().NoError(path.RelayPacket(packet))
			voucher := transfertypes.NewDenom(
				ibctesting.TestCoin.Denom, transfertypes.NewHop(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID),
			).IBCDenom()
			suite.Require().Equal(ibctesting.TestCoin.Amount, balance(user, voucher).Amount)

			// Acknowledging proves the acknowledgement against a counterparty header.
			packet = transfer(path.EndpointA, user, counterpartyUser, counterparty.GetTimeoutHeight())
			suite.Require().NoError(path.RelayPacket(packet))
			suite.Require().False(chainApp.IBCKeeper.ChannelKeeper.HasPacketCommitment(
				chain.GetContext(), packet.SourcePort, packet.SourceChannel, packet.Sequence,
			))
			escrow := transfertypes.GetEscrowAddress(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
			suite.Require().Equal(ibctesting.TestCoin, balance(escrow, ibctesting.TestCoin.Denom))

			// Timing out proves the packet receipt is absent from a counterparty header.
			beforeTimeout := balance(user, ibctesting.TestCoin.Denom)
			timeoutHeight := clienttypes.NewHeight(
				clienttypes.ParseChainID(counterparty.ChainID), uint64(counterparty.ProposedHeader.Height),
			)
			packet = transfer(path.EndpointA, user, counterpartyUser, timeoutHeight)
			suite.Require().NoError(path.EndpointA.UpdateClient())
			suite.Require().NoError(path.EndpointA.TimeoutPacket(packet))
			suite.Require().Equal(beforeTimeout, balance(user, ibctesting.TestCoin.Denom))

			consState, found := chainApp.IBCKeeper.ClientKeeper.GetClientConsensusState(
				chain.GetContext(), path.EndpointA.ClientID, path.EndpointA.GetClientLatestHeight(),
			)
			suite.Require().True(found)
			suite.Require().Equal(valSet.Hash(), []byte(consState.(*ibctm.ConsensusState).NextValidatorsHash))
		})
	}
}

func (suite *AppTestSuite) TestMLDSAClientFixtures() {
	cdc := suite.app.AppCodec()
	if os.Getenv(updateMLDSAFixturesEnv) != "" {
		keys := suite.mlDSAKeys(2)
		clientState, consState := suite.counterpartyClient(keys, fixtureTrustedTime, fixtureTrustingPeriod)
		files := map[string]proto.Message{
			"client_state.json":    clientState,
			"consensus_state.json": consState,
			"header.json":          suite.counterpartyHeader(keys, keys, 2, fixtureTrustedTime.Add(time.Second)),
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

	clientID := suite.createClient(clientState, consState)
	suite.Require().NoError(suite.updateClient(clientID, header))
	_, found := suite.app.IBCKeeper.ClientKeeper.GetClientConsensusState(suite.ctx, clientID, tmHeader.GetHeight())
	suite.Require().True(found)
}
