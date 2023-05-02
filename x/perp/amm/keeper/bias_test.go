package keeper_test

import (
	"testing"
	"time"

	integrationaction "github.com/NibiruChain/nibiru/x/perp/amm/integration/action"
	ammassertion "github.com/NibiruChain/nibiru/x/perp/amm/integration/assertion"
	. "github.com/NibiruChain/nibiru/x/perp/integration/assertion"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/common/asset"
	"github.com/NibiruChain/nibiru/x/common/denoms"
	"github.com/NibiruChain/nibiru/x/common/testutil"
	. "github.com/NibiruChain/nibiru/x/common/testutil/action"
	. "github.com/NibiruChain/nibiru/x/oracle/integration_test/action"
	"github.com/NibiruChain/nibiru/x/perp/amm/types"
	. "github.com/NibiruChain/nibiru/x/perp/integration/action"
)

func createInitMarket() Action {
	pairBtcUsdc := asset.Registry.Pair(denoms.BTC, denoms.USDC)

	return CreateCustomMarket(pairBtcUsdc,
		/* quoteReserve */ sdk.NewDec(1*common.TO_MICRO*common.TO_MICRO),
		/* baseReserve */ sdk.NewDec(1*common.TO_MICRO*common.TO_MICRO),
		types.MarketConfig{
			FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.1"),
			MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			MaxLeverage:            sdk.MustNewDecFromStr("15"),
			MaxOracleSpreadRatio:   sdk.OneDec(), // 100%,
			TradeLimitRatio:        sdk.OneDec(),
		})
}

func TestBiasChangeOnMarket(t *testing.T) {
	alice, bob := testutil.AccAddress(), testutil.AccAddress()
	pairBtcUsdc := asset.Registry.Pair(denoms.BTC, denoms.USDC)
	startBlockTime := time.Now()

	tc := TestCases{
		TC("simple open long position").
			Given(
				createInitMarket(),
				SetBlockTime(startBlockTime),
				SetBlockNumber(1),
				SetOraclePrice(pairBtcUsdc, sdk.MustNewDecFromStr("2.1")),
				FundAccount(alice, sdk.NewCoins(sdk.NewCoin(denoms.USDC, sdk.NewInt(1020)))),
			).
			When(
				OpenPosition(alice, pairBtcUsdc, types.Direction_LONG, sdk.NewInt(1000), sdk.NewDec(10), sdk.ZeroDec()),
			).
			Then(
				ammassertion.MarketShouldBeEqual(pairBtcUsdc,
					ammassertion.Market_BiasShouldBeEqualTo(sdk.MustNewDecFromStr("9999.999900000001000000")), // Bias equal to PositionSize
				),
				PositionShouldBeEqual(alice, pairBtcUsdc, Position_PositionSizeShouldBeEqualTo(sdk.MustNewDecFromStr("9999.999900000001000000"))),
			),

		TC("additional long position").
			Given(
				createInitMarket(),
				SetBlockTime(startBlockTime),
				SetBlockNumber(1),
				SetOraclePrice(pairBtcUsdc, sdk.MustNewDecFromStr("2.1")),
				FundAccount(alice, sdk.NewCoins(sdk.NewCoin(denoms.USDC, sdk.NewInt(2040)))),
				OpenPosition(alice, pairBtcUsdc, types.Direction_LONG, sdk.NewInt(1000), sdk.NewDec(10), sdk.ZeroDec()),
				MoveToNextBlock(),
			).
			When(
				OpenPosition(alice, pairBtcUsdc, types.Direction_LONG, sdk.NewInt(1000), sdk.NewDec(10), sdk.ZeroDec()),
			).
			Then(
				ammassertion.MarketShouldBeEqual(pairBtcUsdc,
					ammassertion.Market_BiasShouldBeEqualTo(sdk.MustNewDecFromStr("19999.999600000008000000")), // Bias equal to PositionSize
				),
				PositionShouldBeEqual(alice, pairBtcUsdc, Position_PositionSizeShouldBeEqualTo(sdk.MustNewDecFromStr("19999.999600000008000000"))),
			),
		TC("simple open short position").
			Given(
				createInitMarket(),
				SetBlockTime(startBlockTime),
				SetBlockNumber(1),
				SetOraclePrice(pairBtcUsdc, sdk.MustNewDecFromStr("2.1")),
				FundAccount(alice, sdk.NewCoins(sdk.NewCoin(denoms.USDC, sdk.NewInt(1020)))),
			).
			When(
				OpenPosition(alice, pairBtcUsdc, types.Direction_SHORT, sdk.NewInt(1000), sdk.NewDec(10), sdk.ZeroDec()),
			).
			Then(
				ammassertion.MarketShouldBeEqual(pairBtcUsdc,
					ammassertion.Market_BiasShouldBeEqualTo(sdk.MustNewDecFromStr("-10000.000100000001000000")), // Bias equal to PositionSize
				),
				PositionShouldBeEqual(alice, pairBtcUsdc, Position_PositionSizeShouldBeEqualTo(sdk.MustNewDecFromStr("-10000.000100000001000000"))),
			),

		TC("additional short position").
			Given(
				createInitMarket(),
				SetBlockTime(startBlockTime),
				SetBlockNumber(1),
				SetOraclePrice(pairBtcUsdc, sdk.MustNewDecFromStr("2.1")),
				FundAccount(alice, sdk.NewCoins(sdk.NewCoin(denoms.USDC, sdk.NewInt(2040)))),
				OpenPosition(alice, pairBtcUsdc, types.Direction_SHORT, sdk.NewInt(1000), sdk.NewDec(10), sdk.ZeroDec()),
				MoveToNextBlock(),
			).
			When(
				OpenPosition(alice, pairBtcUsdc, types.Direction_SHORT, sdk.NewInt(1000), sdk.NewDec(10), sdk.ZeroDec()),
			).
			Then(
				ammassertion.MarketShouldBeEqual(pairBtcUsdc,
					ammassertion.Market_BiasShouldBeEqualTo(sdk.MustNewDecFromStr("-20000.000400000008000000")), // Bias equal to PositionSize
				),
				PositionShouldBeEqual(alice, pairBtcUsdc, Position_PositionSizeShouldBeEqualTo(sdk.MustNewDecFromStr("-20000.000400000008000000"))),
			),
		TC("open long position and close it").
			Given(
				createInitMarket(),
				SetBlockTime(startBlockTime),
				SetBlockNumber(1),
				SetOraclePrice(pairBtcUsdc, sdk.MustNewDecFromStr("2.1")),
				FundAccount(alice, sdk.NewCoins(sdk.NewCoin(denoms.USDC, sdk.NewInt(2040)))),
				OpenPosition(alice, pairBtcUsdc, types.Direction_LONG, sdk.NewInt(1000), sdk.NewDec(10), sdk.ZeroDec()),
				MoveToNextBlock(),
			).
			When(
				ClosePosition(alice, pairBtcUsdc),
			).
			Then(
				ammassertion.MarketShouldBeEqual(pairBtcUsdc,
					ammassertion.Market_BiasShouldBeEqualTo(sdk.ZeroDec()), // Bias equal to PositionSize
				),
				PositionShouldNotExist(alice, pairBtcUsdc),
			),
		TC("open long position and open smaller short").
			Given(
				createInitMarket(),
				SetBlockTime(startBlockTime),
				SetBlockNumber(1),
				SetOraclePrice(pairBtcUsdc, sdk.MustNewDecFromStr("2.1")),
				FundAccount(alice, sdk.NewCoins(sdk.NewCoin(denoms.USDC, sdk.NewInt(2040)))),
				OpenPosition(alice, pairBtcUsdc, types.Direction_LONG, sdk.NewInt(1000), sdk.NewDec(10), sdk.ZeroDec()),
				MoveToNextBlock(),
			).
			When(
				OpenPosition(alice, pairBtcUsdc, types.Direction_SHORT, sdk.NewInt(100), sdk.NewDec(10), sdk.ZeroDec()),
			).
			Then(
				ammassertion.MarketShouldBeEqual(pairBtcUsdc,
					ammassertion.Market_BiasShouldBeEqualTo(sdk.MustNewDecFromStr("8999.999919000000729000")), // Bias equal to PositionSize
				),
				PositionShouldBeEqual(alice, pairBtcUsdc, Position_PositionSizeShouldBeEqualTo(sdk.MustNewDecFromStr("8999.999919000000729000"))),
			),

		TC("2 positions, one long, one short with same amount should set Bias to 0").
			Given(
				createInitMarket(),
				SetBlockTime(startBlockTime),
				SetBlockNumber(1),
				SetOraclePrice(pairBtcUsdc, sdk.MustNewDecFromStr("2.1")),
				FundAccount(alice, sdk.NewCoins(sdk.NewCoin(denoms.USDC, sdk.NewInt(1020)))),
				FundAccount(bob, sdk.NewCoins(sdk.NewCoin(denoms.USDC, sdk.NewInt(1020)))),
			).
			When(
				OpenPosition(alice, pairBtcUsdc, types.Direction_LONG, sdk.NewInt(1000), sdk.NewDec(10), sdk.ZeroDec()),
				OpenPosition(bob, pairBtcUsdc, types.Direction_SHORT, sdk.NewInt(1000), sdk.NewDec(10), sdk.ZeroDec()),
			).
			Then(
				ammassertion.MarketShouldBeEqual(pairBtcUsdc,
					ammassertion.Market_BiasShouldBeEqualTo(sdk.ZeroDec()), // Bias equal to PositionSize
				),
				PositionShouldBeEqual(alice, pairBtcUsdc, Position_PositionSizeShouldBeEqualTo(sdk.MustNewDecFromStr("9999.999900000001000000"))),
				PositionShouldBeEqual(bob, pairBtcUsdc, Position_PositionSizeShouldBeEqualTo(sdk.MustNewDecFromStr("-9999.999900000001000000"))),
			),

		TC("Open long position and liquidate").
			Given(
				createInitMarket(),
				SetLiquidator(bob),
				SetBlockTime(startBlockTime),
				SetBlockNumber(1),
				SetOraclePrice(pairBtcUsdc, sdk.MustNewDecFromStr("2.1")),
				FundAccount(alice, sdk.NewCoins(sdk.NewCoin(denoms.USDC, sdk.NewInt(1020)))),
				FundAccount(bob, sdk.NewCoins(sdk.NewCoin(denoms.USDC, sdk.NewInt(1020)))),
				OpenPosition(alice, pairBtcUsdc, types.Direction_LONG, sdk.NewInt(1000), sdk.NewDec(10), sdk.ZeroDec()),
				MoveToNextBlock(),
				integrationaction.ChangeMaintenanceMarginRatio(pairBtcUsdc, sdk.MustNewDecFromStr("0.2")),
				ChangeLiquidationFeeRatio(sdk.MustNewDecFromStr("0.2")),
			).
			When(
				LiquidatePosition(bob, alice, pairBtcUsdc),
			).Then(
			ammassertion.MarketShouldBeEqual(pairBtcUsdc,
				ammassertion.Market_BiasShouldBeEqualTo(sdk.ZeroDec()), // Bias equal to PositionSize
			),
			PositionShouldNotExist(alice, pairBtcUsdc),
		),
	}

	NewTestSuite(t).WithTestCases(tc...).Run()
}
