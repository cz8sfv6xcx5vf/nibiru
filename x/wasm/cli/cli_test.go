package cli

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/CosmWasm/wasmd/x/wasm"
	wasmcli "github.com/CosmWasm/wasmd/x/wasm/client/cli"
	"github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/testutil/cli"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"

	"github.com/NibiruChain/nibiru/app"
	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/common/asset"
	"github.com/NibiruChain/nibiru/x/common/denoms"
	testutilcli "github.com/NibiruChain/nibiru/x/common/testutil/cli"
	"github.com/NibiruChain/nibiru/x/common/testutil/genesis"
	epochstypes "github.com/NibiruChain/nibiru/x/epochs/types"
	perpv2types "github.com/NibiruChain/nibiru/x/perp/v2/types"
)

// commonArgs is args for CLI test commands.
var commonArgs = []string{
	fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
	fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
	fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(denoms.NIBI, sdk.NewInt(10))).String()),
}

type IntegrationTestSuite struct {
	suite.Suite

	cfg     testutilcli.Config
	network *testutilcli.Network
}

func (s *IntegrationTestSuite) SetupSuite() {
	if testing.Short() {
		s.T().Skip("skipping integration test suite")
	}

	s.T().Log("setting up integration test suite")

	app.SetPrefixes(app.AccountAddressPrefix)

	encodingConfig := app.MakeTestEncodingConfig()
	genesisState := genesis.NewTestGenesisState()
	perpv2Gen := perpv2types.DefaultGenesis()
	perpv2Gen.Markets = []perpv2types.Market{
		{
			Pair:                            asset.Registry.Pair(denoms.ETH, denoms.NUSD),
			Enabled:                         true,
			PriceFluctuationLimitRatio:      sdk.MustNewDecFromStr("0.2"),
			MaintenanceMarginRatio:          sdk.MustNewDecFromStr("0.0625"),
			MaxLeverage:                     sdk.MustNewDecFromStr("15"),
			LatestCumulativePremiumFraction: sdk.ZeroDec(),
			ExchangeFeeRatio:                sdk.MustNewDecFromStr("0.0005"),
			EcosystemFundFeeRatio:           sdk.MustNewDecFromStr("0.0005"),
			LiquidationFeeRatio:             sdk.MustNewDecFromStr("0.001"),
			PartialLiquidationRatio:         sdk.MustNewDecFromStr("0.5"),
			FundingRateEpochId:              epochstypes.ThirtyMinuteEpochID,
			TwapLookbackWindow:              30 * time.Minute,
			PrepaidBadDebt:                  sdk.NewCoin(denoms.NUSD, sdk.NewInt(0)),
		},
	}
	perpv2Gen.Amms = []perpv2types.AMM{
		{
			Pair:            asset.Registry.Pair(denoms.ETH, denoms.NUSD),
			BaseReserve:     sdk.NewDec(10 * common.TO_MICRO),
			QuoteReserve:    sdk.NewDec(10 * common.TO_MICRO),
			SqrtDepth:       common.MustSqrtDec(sdk.NewDec(10 * 10 * common.TO_MICRO * common.TO_MICRO)),
			PriceMultiplier: sdk.NewDec(6000),
			TotalLong:       sdk.ZeroDec(),
			TotalShort:      sdk.ZeroDec(),
		},
	}
	genesisState[perpv2types.ModuleName] = encodingConfig.Marshaler.MustMarshalJSON(perpv2Gen)

	s.cfg = testutilcli.BuildNetworkConfig(genesisState)
	s.network = testutilcli.NewNetwork(s.T(), s.cfg)
	s.Require().NoError(s.network.WaitForNextBlock())
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
	s.network.Cleanup()
}

func (s *IntegrationTestSuite) TestWasmHappyPath() {
	s.requiredDeployedContractsLen(0)

	_, err := s.deployWasmContract("testdata/cw_nameservice.wasm")
	s.Require().NoError(err)

	err = s.network.WaitForNextBlock()
	s.Require().NoError(err)

	s.requiredDeployedContractsLen(1)
}

// deployWasmContract deploys a wasm contract located in path.
func (s *IntegrationTestSuite) deployWasmContract(path string) (uint64, error) {
	val := s.network.Validators[0]
	codec := val.ClientCtx.Codec

	args := []string{
		"--from", val.Address.String(),
		path,
		"--gas", "11000000",
	}
	args = append(args, commonArgs...)

	cmd := wasmcli.StoreCodeCmd()
	out, err := cli.ExecTestCLICmd(val.ClientCtx, cmd, args)
	if err != nil {
		return 0, err
	}

	resp := &sdk.TxResponse{}
	err = codec.UnmarshalJSON(out.Bytes(), resp)
	if err != nil {
		return 0, err
	}

	decodedResult, err := hex.DecodeString(resp.Data)
	if err != nil {
		return 0, err
	}

	respData := sdk.TxMsgData{}
	err = codec.Unmarshal(decodedResult, &respData)
	if err != nil {
		return 0, err
	}

	if len(respData.Data) < 1 {
		return 0, fmt.Errorf("no data found in response")
	}

	var storeCodeResponse wasm.MsgStoreCodeResponse
	err = codec.Unmarshal(respData.Data[0].Data, &storeCodeResponse)
	if err != nil {
		return 0, err
	}

	return storeCodeResponse.CodeID, nil
}

// requiredDeployedContractsLen checks the number of deployed contracts.
func (s *IntegrationTestSuite) requiredDeployedContractsLen(total int) {
	val := s.network.Validators[0]
	var queryCodeResponse types.QueryCodesResponse
	err := testutilcli.ExecQuery(
		val.ClientCtx,
		wasmcli.GetCmdListCode(),
		[]string{},
		&queryCodeResponse,
	)
	s.Require().NoError(err)
	s.Require().Len(queryCodeResponse.CodeInfos, total)
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
