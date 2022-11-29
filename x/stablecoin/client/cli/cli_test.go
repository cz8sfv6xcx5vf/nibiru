package cli_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/NibiruChain/nibiru/simapp"

	"github.com/cosmos/cosmos-sdk/client/flags"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/client/testutil"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/suite"

	"github.com/NibiruChain/nibiru/app"
	"github.com/NibiruChain/nibiru/x/common"
	pftypes "github.com/NibiruChain/nibiru/x/pricefeed/types"
	"github.com/NibiruChain/nibiru/x/stablecoin/client/cli"
	stabletypes "github.com/NibiruChain/nibiru/x/stablecoin/types"
	testutilcli "github.com/NibiruChain/nibiru/x/testutil/cli"
)

type IntegrationTestSuite struct {
	suite.Suite

	cfg     testutilcli.Config
	network *testutilcli.Network
}

// NewPricefeedGen returns an x/pricefeed GenesisState to specify the module parameters.
func NewPricefeedGen() *pftypes.GenesisState {
	pairs := common.AssetPairs{
		common.Pair_NIBI_NUSD, common.Pair_USDC_NUSD,
	}

	defaultGenesis := simapp.PricefeedGenesis()
	defaultGenesis.Params.Pairs = append(defaultGenesis.Params.Pairs, pairs...)
	defaultGenesis.PostedPrices = append(defaultGenesis.PostedPrices, []pftypes.PostedPrice{
		{
			PairID: common.Pair_NIBI_NUSD.String(),
			Oracle: simapp.GenOracleAddress,
			Price:  sdk.NewDec(10),
			Expiry: time.Now().Add(1 * time.Hour),
		},
		{
			PairID: common.Pair_USDC_NUSD.String(),
			Oracle: simapp.GenOracleAddress,
			Price:  sdk.OneDec(),
			Expiry: time.Now().Add(1 * time.Hour),
		},
	}...)

	return &defaultGenesis
}

func (s *IntegrationTestSuite) SetupSuite() {
	/* 	Make test skip if -short is not used:
	All tests: `go test ./...`
	Unit tests only: `go test ./... -short`
	Integration tests only: `go test ./... -run Integration`
	https://stackoverflow.com/a/41407042/13305627 */
	if testing.Short() {
		s.T().Skip("skipping integration test suite")
	}

	s.T().Log("setting up integration test suite")
	app.SetPrefixes(app.AccountAddressPrefix)

	encodingConfig := app.MakeTestEncodingConfig()
	genesisState := simapp.NewTestGenesisStateFromDefault()

	// x/stablecoin genesis state
	stableGen := stabletypes.DefaultGenesis()
	stableGen.Params.IsCollateralRatioValid = true
	stableGen.ModuleAccountBalance = sdk.NewCoin(common.DenomUSDC, sdk.NewInt(10000*common.Precision))
	genesisState[stabletypes.ModuleName] = encodingConfig.Marshaler.MustMarshalJSON(stableGen)

	genesisState[pftypes.ModuleName] = encodingConfig.Marshaler.MustMarshalJSON(NewPricefeedGen())

	s.cfg = testutilcli.BuildNetworkConfig(genesisState)

	s.network = testutilcli.NewNetwork(s.T(), s.cfg)
	_, err := s.network.WaitForHeight(1)
	s.NoError(err)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
	s.network.Cleanup()
}

func (s IntegrationTestSuite) TestMintStableCmd() {
	val := s.network.Validators[0]
	minter := testutilcli.NewAccount(s.network, "minter2")

	s.NoError(testutilcli.FillWalletFromValidator(
		minter,
		sdk.NewCoins(
			sdk.NewInt64Coin(common.DenomNIBI, 100*common.Precision),
			sdk.NewInt64Coin(common.DenomUSDC, 100*common.Precision),
		),
		val,
		s.cfg.BondDenom,
	))

	commonArgs := []string{
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(common.DenomNIBI, sdk.NewInt(10))).String()),
	}

	testCases := []struct {
		name string
		args []string

		expectedStable sdk.Int
		expectErr      bool
		respType       proto.Message
		expectedCode   uint32
	}{
		{
			name: "Mint correct amount",
			args: append([]string{
				"1000000unusd",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, "minter2")}, commonArgs...),
			expectedStable: sdk.NewInt(1 * common.Precision),
			expectErr:      false,
			respType:       &sdk.TxResponse{},
			expectedCode:   0,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.MintStableCmd()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.NoError(err, out.String())
				s.NoError(
					clientCtx.Codec.UnmarshalJSON(out.Bytes(), tc.respType), out.String())

				txResp := tc.respType.(*sdk.TxResponse)
				err = val.ClientCtx.Codec.UnmarshalJSON(out.Bytes(), txResp)
				s.NoError(err)
				s.Require().Equal(tc.expectedCode, txResp.Code, out.String())

				resp, err := banktestutil.QueryBalancesExec(clientCtx, minter)
				s.NoError(err)

				var balRes banktypes.QueryAllBalancesResponse
				err = val.ClientCtx.Codec.UnmarshalJSON(resp.Bytes(), &balRes)
				s.NoError(err)

				s.Require().Equal(
					balRes.Balances.AmountOf(common.DenomNUSD), tc.expectedStable)
			}
		})
	}
}

func (s IntegrationTestSuite) TestBurnStableCmd() {
	val := s.network.Validators[0]
	burner := testutilcli.NewAccount(s.network, "burn")
	s.NoError(testutilcli.FillWalletFromValidator(
		burner,
		sdk.NewCoins(
			sdk.NewInt64Coin(s.cfg.BondDenom, 20_000),
			sdk.NewInt64Coin(common.DenomNUSD, 50*common.Precision),
		),
		val,
		s.cfg.BondDenom,
	))

	s.NoError(s.network.WaitForNextBlock())

	defaultBondCoinsString := sdk.NewCoins(sdk.NewCoin(common.DenomNIBI, sdk.NewInt(10))).String()
	commonArgs := []string{
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
		fmt.Sprintf("--%s=%s", flags.FlagFees, defaultBondCoinsString),
	}

	testCases := []struct {
		name string
		args []string

		expectedStable   sdk.Int
		expectedColl     sdk.Int
		expectedGov      sdk.Int
		expectedTreasury sdk.Coins
		expectedEf       sdk.Coins
		expectErr        bool
		respType         proto.Message
		expectedCode     uint32
	}{
		{
			name: "Burn at 100% collRatio",
			args: append([]string{
				"50000000unusd",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, "burn")}, commonArgs...),
			expectedStable:   sdk.ZeroInt(),
			expectedColl:     sdk.NewInt(50*common.Precision - 100_000), // Collateral minus 0,02% fees
			expectedGov:      sdk.NewInt(19_990),
			expectedTreasury: sdk.NewCoins(sdk.NewInt64Coin(common.DenomUSDC, 50_000)),
			expectedEf:       sdk.NewCoins(sdk.NewInt64Coin(common.DenomUSDC, 50_000)),
			expectErr:        false,
			respType:         &sdk.TxResponse{},
			expectedCode:     0,
		},
		// {
		// 	name: "Burn at 90% collRatio",
		// 	args: append([]string{
		// 		"100000000unusd",
		// 		fmt.Sprintf("--%s=%s", flags.FlagFrom, "burn")}, commonArgs...),
		// 	expectedStable: sdk.NewInt(0),
		// 	expectedColl:   sdk.NewInt(90* common.Precision),
		// 	expectedGov:    sdk.NewInt(1* common.Precision),
		// 	expectErr:      false,
		// 	respType:       &sdk.TxResponse{},
		// 	expectedCode:   0,
		// },
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.BurnStableCmd()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.NoError(err, out.String())
				s.NoError(
					clientCtx.Codec.UnmarshalJSON(out.Bytes(), tc.respType),
					out.String(),
				)

				txResp := tc.respType.(*sdk.TxResponse)
				err = val.ClientCtx.Codec.UnmarshalJSON(out.Bytes(), txResp)
				s.NoError(err)
				s.Require().Equal(tc.expectedCode, txResp.Code, out.String())

				resp, err := banktestutil.QueryBalancesExec(clientCtx, burner)
				s.NoError(err)

				var balRes banktypes.QueryAllBalancesResponse
				err = val.ClientCtx.Codec.UnmarshalJSON(resp.Bytes(), &balRes)
				s.NoError(err)

				s.Require().Equal(
					tc.expectedColl, balRes.Balances.AmountOf(common.DenomUSDC))
				s.Require().Equal(
					tc.expectedGov, balRes.Balances.AmountOf(common.DenomNIBI))
				s.Require().Equal(
					tc.expectedStable, balRes.Balances.AmountOf(common.DenomNUSD))

				// Query treasury pool balance
				resp, err = banktestutil.QueryBalancesExec(
					clientCtx, types.NewModuleAddress(common.TreasuryPoolModuleAccount))
				s.NoError(err)
				err = val.ClientCtx.Codec.UnmarshalJSON(resp.Bytes(), &balRes)
				s.NoError(err)

				s.Require().Equal(
					tc.expectedTreasury, balRes.Balances)

				// Query ecosystem fund balance
				resp, err = banktestutil.QueryBalancesExec(
					clientCtx,
					types.NewModuleAddress(stabletypes.StableEFModuleAccount))
				s.NoError(err)
				err = val.ClientCtx.Codec.UnmarshalJSON(resp.Bytes(), &balRes)
				s.NoError(err)

				s.Require().Equal(
					tc.expectedEf, balRes.Balances)
			}
		})
	}
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
