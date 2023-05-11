package keeper_test

import (
	"testing"
	"time"

	"github.com/NibiruChain/collections"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/common/asset"
	"github.com/NibiruChain/nibiru/x/common/denoms"
	"github.com/NibiruChain/nibiru/x/common/testutil"
	testutilevents "github.com/NibiruChain/nibiru/x/common/testutil"
	"github.com/NibiruChain/nibiru/x/common/testutil/testapp"
	perpammtypes "github.com/NibiruChain/nibiru/x/perp/amm/types"
	"github.com/NibiruChain/nibiru/x/perp/keeper/v1"
	types "github.com/NibiruChain/nibiru/x/perp/types/v1"
)

func TestExecuteFullLiquidation(t *testing.T) {
	// constants for this suite
	tokenPair := asset.MustNewPair("BTC:NUSD")

	traderAddr := testutilevents.AccAddress()

	type test struct {
		positionSide              perpammtypes.Direction
		quoteAmount               sdk.Int
		leverage                  sdk.Dec
		baseAssetLimit            sdk.Dec
		liquidationFee            sdk.Dec
		traderFunds               sdk.Coin
		expectedLiquidatorBalance sdk.Coin
		expectedPerpEFBalance     sdk.Coin
	}

	testCases := map[string]test{
		"happy path - Buy": {
			positionSide:   perpammtypes.Direction_LONG,
			quoteAmount:    sdk.NewInt(5_000_000),
			leverage:       sdk.OneDec(),
			baseAssetLimit: sdk.ZeroDec(),
			liquidationFee: sdk.MustNewDecFromStr("0.1"),
			// There's a 20 bps tx fee on open position.
			// This tx fee is split 50/50 bw the PerpEF and Treasury.
			// txFee = exchangedQuote * 20 bps = 100
			traderFunds: sdk.NewInt64Coin("NUSD", 5_010_000),
			// feeToLiquidator
			//   = positionResp.ExchangedNotionalValue * liquidationFee / 2
			//   = 5_000_000 * 0.1 / 2 = 250_000
			expectedLiquidatorBalance: sdk.NewInt64Coin("NUSD", 250_000),
			// startingBalance = 1* common.TO_MICRO
			// perpEFBalance = startingBalance + openPositionDelta + liquidateDelta
			expectedPerpEFBalance: sdk.NewInt64Coin("NUSD", 5755000),
		},
		"happy path - Sell": {
			positionSide: perpammtypes.Direction_SHORT,
			quoteAmount:  sdk.NewInt(5_000_000),
			// There's a 20 bps tx fee on open position.
			// This tx fee is split 50/50 bw the PerpEF and Treasury.
			// txFee = exchangedQuote * 20 bps = 100
			traderFunds:    sdk.NewInt64Coin("NUSD", 5_010_000),
			leverage:       sdk.OneDec(),
			baseAssetLimit: sdk.ZeroDec(),
			liquidationFee: sdk.MustNewDecFromStr("0.123123"),
			// feeToLiquidator
			//   = positionResp.ExchangedNotionalValue * liquidationFee / 2
			//   = 50_000 * 0.123123 / 2 = 3078.025 → 3078
			expectedLiquidatorBalance: sdk.NewInt64Coin("NUSD", 307_808),
			// startingBalance = 1* common.TO_MICRO
			// perpEFBalance = startingBalance + openPositionDelta + liquidateDelta
			expectedPerpEFBalance: sdk.NewInt64Coin("NUSD", 5_697_192),
		},
	}

	for name, testCase := range testCases {
		tc := testCase
		t.Run(name, func(t *testing.T) {
			nibiruApp, ctx := testapp.NewNibiruTestAppAndContext(true)
			ctx = ctx.WithBlockTime(time.Now())
			perpKeeper := &nibiruApp.PerpKeeper

			t.Log("create market")
			perpammKeeper := &nibiruApp.PerpAmmKeeper
			assert.NoError(t, perpammKeeper.CreatePool(
				ctx,
				tokenPair,
				/* quoteReserves */ sdk.NewDec(500*common.TO_MICRO),
				/* baseReserves */ sdk.NewDec(500*common.TO_MICRO),
				perpammtypes.MarketConfig{
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.9"),
					FluctuationLimitRatio:  sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.1"),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.MustNewDecFromStr("15"),
				},
				sdk.NewDec(2),
			))
			require.True(t, perpammKeeper.ExistsPool(ctx, tokenPair))

			nibiruApp.OracleKeeper.SetPrice(ctx, tokenPair, sdk.NewDec(2))

			t.Log("set perpkeeper params")
			params := types.DefaultParams()
			perpKeeper.SetParams(ctx, types.NewParams(
				params.Stopped,
				params.FeePoolFeeRatio,
				params.EcosystemFundFeeRatio,
				tc.liquidationFee,
				params.PartialLiquidationRatio,
				"hour",
				15*time.Minute,
			))
			keeper.SetPairMetadata(nibiruApp.PerpKeeper, ctx, types.PairMetadata{
				Pair:                            tokenPair,
				LatestCumulativePremiumFraction: sdk.OneDec(),
			})

			t.Log("Fund trader account with sufficient quote")
			var err error
			err = testapp.FundAccount(nibiruApp.BankKeeper, ctx, traderAddr,
				sdk.NewCoins(tc.traderFunds))
			require.NoError(t, err)

			t.Log("increment block height and time for TWAP calculation")
			ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1).
				WithBlockTime(time.Now().Add(time.Minute))

			t.Log("Open position")
			positionResp, err := nibiruApp.PerpKeeper.OpenPosition(
				ctx, tokenPair, tc.positionSide, traderAddr, tc.quoteAmount, tc.leverage, tc.baseAssetLimit)
			require.NoError(t, err)

			t.Log("Artificially populate Vault and PerpEF to prevent bankKeeper errors")
			startingModuleFunds := sdk.NewCoins(sdk.NewInt64Coin(
				tokenPair.QuoteDenom(), 1*common.TO_MICRO))
			assert.NoError(t, testapp.FundModuleAccount(
				nibiruApp.BankKeeper, ctx, types.VaultModuleAccount, startingModuleFunds))
			assert.NoError(t, testapp.FundModuleAccount(
				nibiruApp.BankKeeper, ctx, types.PerpEFModuleAccount, startingModuleFunds))

			t.Log("Liquidate the (entire) position")
			liquidatorAddr := testutilevents.AccAddress()
			liquidationResp, err := nibiruApp.PerpKeeper.ExecuteFullLiquidation(ctx, liquidatorAddr, positionResp.Position)
			require.NoError(t, err)

			t.Log("Check correctness of new position")
			newPosition, err := nibiruApp.PerpKeeper.Positions.Get(ctx, collections.Join(tokenPair, traderAddr))
			require.ErrorIs(t, err, collections.ErrNotFound)
			require.Empty(t, newPosition)

			t.Log("Check correctness of liquidation fee distributions")
			liquidatorBalance := nibiruApp.BankKeeper.GetBalance(
				ctx, liquidatorAddr, tokenPair.QuoteDenom())
			assert.EqualValues(t, tc.expectedLiquidatorBalance, liquidatorBalance)

			perpEFAddr := nibiruApp.AccountKeeper.GetModuleAddress(
				types.PerpEFModuleAccount)
			perpEFBalance := nibiruApp.BankKeeper.GetBalance(
				ctx, perpEFAddr, tokenPair.QuoteDenom())
			require.EqualValues(t, tc.expectedPerpEFBalance, perpEFBalance)

			t.Log("check emitted events")
			newMarkPrice, err := perpammKeeper.GetMarkPrice(ctx, tokenPair)
			require.NoError(t, err)
			testutilevents.RequireHasTypedEvent(t, ctx, &types.PositionLiquidatedEvent{
				Pair:                  tokenPair,
				TraderAddress:         traderAddr.String(),
				ExchangedQuoteAmount:  liquidationResp.PositionResp.ExchangedNotionalValue,
				ExchangedPositionSize: liquidationResp.PositionResp.ExchangedPositionSize,
				LiquidatorAddress:     liquidatorAddr.String(),
				FeeToLiquidator:       sdk.NewCoin(tokenPair.QuoteDenom(), liquidationResp.FeeToLiquidator),
				FeeToEcosystemFund:    sdk.NewCoin(tokenPair.QuoteDenom(), liquidationResp.FeeToPerpEcosystemFund),
				BadDebt:               sdk.NewCoin(tokenPair.QuoteDenom(), liquidationResp.BadDebt),
				Margin:                sdk.NewCoin(tokenPair.QuoteDenom(), sdk.ZeroInt()),
				PositionNotional:      liquidationResp.PositionResp.PositionNotional,
				PositionSize:          sdk.ZeroDec(),
				UnrealizedPnl:         liquidationResp.PositionResp.UnrealizedPnlAfter,
				MarkPrice:             newMarkPrice,
				BlockHeight:           ctx.BlockHeight(),
				BlockTimeMs:           ctx.BlockTime().UnixMilli(),
			})
		})
	}
}

func TestExecutePartialLiquidation(t *testing.T) {
	// constants for this suite
	tokenPair := asset.MustNewPair("xxx:yyy")

	traderAddr := testutilevents.AccAddress()
	partialLiquidationRatio := sdk.MustNewDecFromStr("0.4")

	testCases := []struct {
		name           string
		side           perpammtypes.Direction
		quote          sdk.Int
		leverage       sdk.Dec
		baseLimit      sdk.Dec
		liquidationFee sdk.Dec
		traderFunds    sdk.Coin

		expectedLiquidatorBalance sdk.Coin
		expectedPerpEFBalance     sdk.Coin
		expectedPositionSize      sdk.Dec
		expectedMarginRemaining   sdk.Dec
	}{
		{
			name:           "happy path - Buy",
			side:           perpammtypes.Direction_LONG,
			quote:          sdk.NewInt(5_000_000),
			leverage:       sdk.OneDec(),
			baseLimit:      sdk.ZeroDec(),
			liquidationFee: sdk.MustNewDecFromStr("0.1"),
			traderFunds:    sdk.NewInt64Coin("yyy", 5_010_000),
			/* expectedPositionSize =  */
			// 24_999.9999999875000000001 * 0.6
			expectedPositionSize:    sdk.MustNewDecFromStr("1499999.999625000000093750"),
			expectedMarginRemaining: sdk.MustNewDecFromStr("4799999.999970000000003000"), // approx 2k less but slippage

			// feeToLiquidator
			//   = positionResp.ExchangedNotionalValue * 0.4 * liquidationFee / 2
			//   = 50_000 * 0.4 * 0.1 / 2 = 1_000
			expectedLiquidatorBalance: sdk.NewInt64Coin("yyy", 100_000),

			// startingBalance = 1* common.TO_MICRO
			// perpEFBalance = startingBalance + openPositionDelta + liquidateDelta
			expectedPerpEFBalance: sdk.NewInt64Coin("yyy", 1_105_000),
		},
		{
			name:           "happy path - Sell",
			side:           perpammtypes.Direction_SHORT,
			quote:          sdk.NewInt(5_000_000),
			leverage:       sdk.OneDec(),
			baseLimit:      sdk.ZeroDec(),
			liquidationFee: sdk.MustNewDecFromStr("0.1"),
			traderFunds:    sdk.NewInt64Coin("yyy", 5_010_000),
			// There's a 20 bps tx fee on open position.
			// This tx fee is split 50/50 bw the PerpEF and Treasury.
			// exchangedQuote * 20 bps = 100

			expectedPositionSize:    sdk.MustNewDecFromStr("-1500000.000575000000103750"), // ~-25k * 0.6
			expectedMarginRemaining: sdk.MustNewDecFromStr("4800000.000069999999993000"),  // approx 2k less but slippage

			// feeToLiquidator
			//   = positionResp.ExchangedNotionalValue * 0.4 * liquidationFee / 2
			//   = 50_000 * 0.4 * 0.1 / 2 = 1_000
			expectedLiquidatorBalance: sdk.NewInt64Coin("yyy", 100_000),

			// startingBalance = 1* common.TO_MICRO
			// perpEFBalance = startingBalance + openPositionDelta + liquidateDelta
			expectedPerpEFBalance: sdk.NewInt64Coin("yyy", 1_105_000),
		},
	}

	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			nibiruApp, ctx := testapp.NewNibiruTestAppAndContext(true)
			ctx = ctx.WithBlockTime(time.Now())

			t.Log("Set market defined by pair on PerpAmmKeeper")
			perpammKeeper := &nibiruApp.PerpAmmKeeper
			assert.NoError(t, perpammKeeper.CreatePool(
				ctx,
				tokenPair,
				/* quoteReserves */ sdk.NewDec(10_000*common.TO_MICRO*common.TO_MICRO),
				/* baseReserves */ sdk.NewDec(10_000*common.TO_MICRO*common.TO_MICRO),
				perpammtypes.MarketConfig{
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.9"),
					FluctuationLimitRatio:  sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.1"),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.MustNewDecFromStr("15"),
				},
				sdk.NewDec(2),
			))
			require.True(t, perpammKeeper.ExistsPool(ctx, tokenPair))

			t.Log("Set market defined by pair on PerpKeeper")
			perpKeeper := &nibiruApp.PerpKeeper
			params := types.DefaultParams()

			perpKeeper.SetParams(ctx, types.NewParams(
				params.Stopped,
				params.FeePoolFeeRatio,
				params.EcosystemFundFeeRatio,
				tc.liquidationFee,
				partialLiquidationRatio,
				"hour",
				15*time.Minute,
			))

			keeper.SetPairMetadata(nibiruApp.PerpKeeper, ctx, types.PairMetadata{
				Pair:                            tokenPair,
				LatestCumulativePremiumFraction: sdk.OneDec(),
			})

			t.Log("Fund trader account with sufficient quote")
			var err error
			err = testapp.FundAccount(nibiruApp.BankKeeper, ctx, traderAddr,
				sdk.NewCoins(tc.traderFunds))
			require.NoError(t, err)

			t.Log("increment block height and time for TWAP calculation")
			ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1).
				WithBlockTime(time.Now().Add(time.Minute))

			t.Log("Open position")
			positionResp, err := nibiruApp.PerpKeeper.OpenPosition(
				ctx, tokenPair, tc.side, traderAddr, tc.quote, tc.leverage, tc.baseLimit)
			require.NoError(t, err)

			t.Log("Artificially populate Vault and PerpEF to prevent bankKeeper errors")
			startingModuleFunds := sdk.NewCoins(sdk.NewInt64Coin(
				tokenPair.QuoteDenom(), 1*common.TO_MICRO))
			assert.NoError(t, testapp.FundModuleAccount(
				nibiruApp.BankKeeper, ctx, types.VaultModuleAccount, startingModuleFunds))
			assert.NoError(t, testapp.FundModuleAccount(
				nibiruApp.BankKeeper, ctx, types.PerpEFModuleAccount, startingModuleFunds))

			t.Log("Liquidate the (partial) position")
			liquidator := testutilevents.AccAddress()
			liquidationResp, err := nibiruApp.PerpKeeper.ExecutePartialLiquidation(ctx, liquidator, positionResp.Position)
			require.NoError(t, err)

			t.Log("Check correctness of new position")
			newPosition, err := nibiruApp.PerpKeeper.Positions.Get(ctx, collections.Join(tokenPair, traderAddr))
			require.NoError(t, err)
			assert.Equal(t, tc.expectedPositionSize, newPosition.Size_)
			assert.Equal(t, tc.expectedMarginRemaining, newPosition.Margin)

			t.Log("Check liquidator balance")
			assert.EqualValues(t,
				tc.expectedLiquidatorBalance.String(),
				nibiruApp.BankKeeper.GetBalance(
					ctx,
					liquidator,
					tokenPair.QuoteDenom(),
				).String(),
			)

			t.Log("Check PerpEF balance")
			perpEFAddr := nibiruApp.AccountKeeper.GetModuleAddress(
				types.PerpEFModuleAccount)
			assert.EqualValues(t, perpEFAddr, nibiruApp.AccountKeeper.GetModuleAddress(types.PerpEFModuleAccount))
			assert.EqualValues(t,
				tc.expectedPerpEFBalance.String(),
				nibiruApp.BankKeeper.GetBalance(
					ctx,
					nibiruApp.AccountKeeper.GetModuleAddress(types.PerpEFModuleAccount),
					tokenPair.QuoteDenom(),
				).String(),
			)

			t.Log("check emitted events")
			newMarkPrice, err := perpammKeeper.GetMarkPrice(ctx, tokenPair)
			require.NoError(t, err)
			testutilevents.RequireHasTypedEvent(t, ctx, &types.PositionLiquidatedEvent{
				Pair:                  tokenPair,
				TraderAddress:         traderAddr.String(),
				ExchangedQuoteAmount:  liquidationResp.PositionResp.ExchangedNotionalValue,
				ExchangedPositionSize: liquidationResp.PositionResp.ExchangedPositionSize,
				LiquidatorAddress:     liquidator.String(),
				FeeToLiquidator:       sdk.NewCoin(tokenPair.QuoteDenom(), liquidationResp.FeeToLiquidator),
				FeeToEcosystemFund:    sdk.NewCoin(tokenPair.QuoteDenom(), liquidationResp.FeeToPerpEcosystemFund),
				BadDebt:               sdk.NewCoin(tokenPair.QuoteDenom(), liquidationResp.BadDebt),
				Margin:                sdk.NewCoin(tokenPair.QuoteDenom(), newPosition.Margin.RoundInt()),
				PositionNotional:      liquidationResp.PositionResp.PositionNotional,
				PositionSize:          newPosition.Size_,
				UnrealizedPnl:         liquidationResp.PositionResp.UnrealizedPnlAfter,
				MarkPrice:             newMarkPrice,
				BlockHeight:           ctx.BlockHeight(),
				BlockTimeMs:           ctx.BlockTime().UnixMilli(),
			})
		})
	}
}
func TestMultiLiquidate(t *testing.T) {
	tests := []struct {
		name string

		liquidator sdk.AccAddress

		positions      []types.Position
		isLiquidatable []bool
		expectedErr    error
	}{
		{
			name:       "success",
			liquidator: testutil.AccAddress(),
			positions: []types.Position{
				// liquidated
				{
					TraderAddress:                   testutil.AccAddress().String(),
					Pair:                            asset.Registry.Pair(denoms.BTC, denoms.NUSD),
					Size_:                           sdk.OneDec(),
					Margin:                          sdk.OneDec(),
					OpenNotional:                    sdk.NewDec(2),
					LatestCumulativePremiumFraction: sdk.ZeroDec(),
					BlockNumber:                     1,
				},
				// not liquidated
				{
					TraderAddress:                   testutil.AccAddress().String(),
					Pair:                            asset.Registry.Pair(denoms.BTC, denoms.NUSD),
					Size_:                           sdk.OneDec(),
					Margin:                          sdk.OneDec(),
					OpenNotional:                    sdk.NewDec(1),
					LatestCumulativePremiumFraction: sdk.ZeroDec(),
					BlockNumber:                     1,
				},
				// liquidated
				{
					TraderAddress:                   testutil.AccAddress().String(),
					Pair:                            asset.Registry.Pair(denoms.BTC, denoms.NUSD),
					Size_:                           sdk.OneDec(),
					Margin:                          sdk.OneDec(),
					OpenNotional:                    sdk.NewDec(2),
					LatestCumulativePremiumFraction: sdk.ZeroDec(),
					BlockNumber:                     1,
				},
			},
			isLiquidatable: []bool{true, false, true},
			expectedErr:    nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			app, ctx := testapp.NewNibiruTestAppAndContext(true)
			ctx = ctx.WithBlockTime(time.Now())
			setLiquidator(ctx, app.PerpKeeper, tc.liquidator)
			msgServer := keeper.NewMsgServerImpl(app.PerpKeeper)

			t.Log("create market")
			assert.NoError(t, app.PerpAmmKeeper.CreatePool(
				/* ctx */ ctx,
				/* pair */ asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				/* quoteReserve */ sdk.NewDec(1*common.TO_MICRO),
				/* baseReserve */ sdk.NewDec(1*common.TO_MICRO),
				perpammtypes.MarketConfig{
					TradeLimitRatio:        sdk.OneDec(),
					FluctuationLimitRatio:  sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.MustNewDecFromStr("15"),
				},
				sdk.OneDec(),
			))

			t.Log("set pair metadata")
			keeper.SetPairMetadata(app.PerpKeeper, ctx, types.PairMetadata{
				Pair:                            asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				LatestCumulativePremiumFraction: sdk.ZeroDec(),
			})
			ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1).WithBlockTime(time.Now().Add(time.Minute))

			t.Log("set oracle price")
			app.OracleKeeper.SetPrice(ctx, asset.Registry.Pair(denoms.BTC, denoms.NUSD), sdk.OneDec())

			t.Log("create position")
			liquidations := make([]*types.MsgMultiLiquidate_Liquidation, len(tc.positions))
			for i, pos := range tc.positions {
				keeper.SetPosition(app.PerpKeeper, ctx, pos)
				require.NoError(t, testapp.FundModuleAccount(app.BankKeeper, ctx, types.VaultModuleAccount, sdk.NewCoins(sdk.NewInt64Coin(pos.Pair.QuoteDenom(), 1))))

				liquidations[i] = &types.MsgMultiLiquidate_Liquidation{
					Pair:   pos.Pair,
					Trader: pos.TraderAddress,
				}
			}

			resp, err := msgServer.MultiLiquidate(sdk.WrapSDKContext(ctx), &types.MsgMultiLiquidate{
				Sender:       tc.liquidator.String(),
				Liquidations: liquidations,
			})

			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
				require.Nil(t, resp)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, resp)

			for i, p := range tc.positions {
				traderAddr := sdk.MustAccAddressFromBech32(p.TraderAddress)
				position, err := app.PerpKeeper.Positions.Get(ctx, collections.Join(p.Pair, traderAddr))
				if tc.isLiquidatable[i] {
					require.Error(t, err)
					assert.True(t, resp.Liquidations[i].Success)
				} else {
					require.NoError(t, err)
					assert.False(t, position.Size_.IsZero())
					assert.False(t, resp.Liquidations[i].Success)
				}
			}
		})
	}
}
