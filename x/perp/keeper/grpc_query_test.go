package keeper_test

import (
	"testing"
	"time"

	"github.com/NibiruChain/nibiru/x/testutil"
	vpooltypes "github.com/NibiruChain/nibiru/x/vpool/types"

	"github.com/NibiruChain/nibiru/simapp"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/perp/keeper"
	"github.com/NibiruChain/nibiru/x/perp/types"
)

func initAppVpools(
	t *testing.T, quoteAssetReserve sdk.Dec, baseAssetReserve sdk.Dec,
) (sdk.Context, *simapp.NibiruTestApp, types.QueryServer) {
	t.Log("initialize app and keeper")
	nibiruApp, ctx := simapp.NewTestNibiruAppAndContext(true)
	perpKeeper := &nibiruApp.PerpKeeper
	vpoolKeeper := &nibiruApp.VpoolKeeper
	queryServer := keeper.NewQuerier(*perpKeeper)

	t.Log("initialize vpool and pair")
	vpoolKeeper.CreatePool(
		ctx,
		common.Pair_BTC_NUSD,
		quoteAssetReserve,
		baseAssetReserve,
		vpooltypes.VpoolConfig{
			TradeLimitRatio:        sdk.OneDec(),
			FluctuationLimitRatio:  sdk.OneDec(),
			MaxOracleSpreadRatio:   sdk.OneDec(),
			MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			MaxLeverage:            sdk.MustNewDecFromStr("15"),
		},
	)
	setPairMetadata(nibiruApp.PerpKeeper, ctx, types.PairMetadata{
		Pair:                            common.Pair_BTC_NUSD,
		LatestCumulativePremiumFraction: sdk.ZeroDec(),
	})
	vpoolKeeper.CreatePool(
		ctx,
		common.Pair_ETH_NUSD,
		/* quoteReserve */ sdk.MustNewDecFromStr("100000"),
		/* baseReserve */ sdk.MustNewDecFromStr("100000"),
		vpooltypes.VpoolConfig{
			TradeLimitRatio:        sdk.OneDec(),
			FluctuationLimitRatio:  sdk.OneDec(),
			MaxOracleSpreadRatio:   sdk.OneDec(),
			MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			MaxLeverage:            sdk.MustNewDecFromStr("15"),
		},
	)
	setPairMetadata(nibiruApp.PerpKeeper, ctx, types.PairMetadata{
		Pair:                            common.Pair_ETH_NUSD,
		LatestCumulativePremiumFraction: sdk.ZeroDec(),
	})
	vpoolKeeper.CreatePool(
		ctx,
		common.Pair_NIBI_NUSD,
		/* quoteReserve */ sdk.MustNewDecFromStr("100000"),
		/* baseReserve */ sdk.MustNewDecFromStr("100000"),
		vpooltypes.VpoolConfig{
			TradeLimitRatio:        sdk.OneDec(),
			FluctuationLimitRatio:  sdk.OneDec(),
			MaxOracleSpreadRatio:   sdk.OneDec(),
			MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			MaxLeverage:            sdk.MustNewDecFromStr("15"),
		},
	)
	setPairMetadata(nibiruApp.PerpKeeper, ctx, types.PairMetadata{
		Pair:                            common.Pair_NIBI_NUSD,
		LatestCumulativePremiumFraction: sdk.ZeroDec(),
	})
	return ctx, nibiruApp, queryServer
}

func TestQueryPosition(t *testing.T) {
	tests := []struct {
		name            string
		initialPosition *types.Position

		quoteAssetReserve sdk.Dec
		baseAssetReserve  sdk.Dec

		expectedPositionNotional sdk.Dec
		expectedUnrealizedPnl    sdk.Dec
		expectedMarginRatio      sdk.Dec
	}{
		{
			name: "positive PnL",
			initialPosition: &types.Position{
				Pair:                            common.Pair_BTC_NUSD,
				Size_:                           sdk.NewDec(10),
				OpenNotional:                    sdk.NewDec(10),
				Margin:                          sdk.NewDec(1),
				BlockNumber:                     1,
				LatestCumulativePremiumFraction: sdk.ZeroDec(),
			},
			quoteAssetReserve: sdk.NewDec(1 * common.Precision),
			baseAssetReserve:  sdk.NewDec(500_000),

			expectedPositionNotional: sdk.MustNewDecFromStr("19.999600007999840003"),
			expectedUnrealizedPnl:    sdk.MustNewDecFromStr("9.999600007999840003"),
			expectedMarginRatio:      sdk.MustNewDecFromStr("0.549991"),
		},
		{
			name: "negative PnL, positive margin ratio",
			initialPosition: &types.Position{
				Pair:                            common.Pair_BTC_NUSD,
				Size_:                           sdk.NewDec(10),
				OpenNotional:                    sdk.NewDec(10),
				Margin:                          sdk.NewDec(1),
				BlockNumber:                     1,
				LatestCumulativePremiumFraction: sdk.ZeroDec(),
			},
			quoteAssetReserve: sdk.NewDec(1 * common.Precision),
			baseAssetReserve:  sdk.NewDec(1 * common.Precision),

			expectedPositionNotional: sdk.MustNewDecFromStr("9.99990000099999"),
			expectedUnrealizedPnl:    sdk.MustNewDecFromStr("-0.00009999900001"),
			expectedMarginRatio:      sdk.MustNewDecFromStr("0.099991"),
		},
		{
			name: "negative PnL, negative margin ratio",
			initialPosition: &types.Position{
				Pair:                            common.Pair_BTC_NUSD,
				Size_:                           sdk.NewDec(10),
				OpenNotional:                    sdk.NewDec(10),
				Margin:                          sdk.NewDec(1),
				BlockNumber:                     1,
				LatestCumulativePremiumFraction: sdk.ZeroDec(),
			},
			quoteAssetReserve: sdk.NewDec(500_000),
			baseAssetReserve:  sdk.NewDec(1 * common.Precision),

			expectedPositionNotional: sdk.MustNewDecFromStr("4.999950000499995"),
			expectedUnrealizedPnl:    sdk.MustNewDecFromStr("-5.000049999500005"),
			expectedMarginRatio:      sdk.MustNewDecFromStr("-0.800018"),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Log("initialize trader address")
			traderAddr := testutil.AccAddress()
			tc.initialPosition.TraderAddress = traderAddr.String()

			t.Log("initialize app and keeper")
			ctx, app, queryServer := initAppVpools(t, tc.quoteAssetReserve, tc.baseAssetReserve)

			t.Log("initialize position")
			setPosition(app.PerpKeeper, ctx, *tc.initialPosition)

			t.Log("query position")
			ctx = ctx.WithBlockTime(ctx.BlockTime().Add(time.Second))
			resp, err := queryServer.QueryPosition(
				sdk.WrapSDKContext(ctx),
				&types.QueryPositionRequest{
					Trader:    traderAddr.String(),
					TokenPair: common.Pair_BTC_NUSD.String(),
				},
			)
			require.NoError(t, err)

			t.Log("assert response")
			assert.EqualValues(t, tc.initialPosition, resp.Position)

			assert.Equal(t, tc.expectedPositionNotional, resp.PositionNotional)
			assert.Equal(t, tc.expectedUnrealizedPnl, resp.UnrealizedPnl)
			assert.Equal(t, tc.expectedMarginRatio, resp.MarginRatioMark)
			// assert.Equal(t, tc.expectedMarginRatioIndex, resp.MarginRatioIndex)
			// TODO https://github.com/NibiruChain/nibiru/issues/809
		})
	}
}

func TestQueryPositions(t *testing.T) {
	tests := []struct {
		name      string
		Positions []*types.Position
	}{
		{
			name: "positive PnL",
			Positions: []*types.Position{
				{
					Pair:                            common.Pair_BTC_NUSD,
					Size_:                           sdk.NewDec(10),
					OpenNotional:                    sdk.NewDec(10),
					Margin:                          sdk.NewDec(1),
					BlockNumber:                     1,
					LatestCumulativePremiumFraction: sdk.ZeroDec(),
				},
				{
					Pair:                            common.Pair_ETH_NUSD,
					Size_:                           sdk.NewDec(10),
					OpenNotional:                    sdk.NewDec(10),
					Margin:                          sdk.NewDec(1),
					BlockNumber:                     1,
					LatestCumulativePremiumFraction: sdk.ZeroDec(),
				},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Log("initialize trader address")
			traderAddr := testutil.AccAddress()

			tc.Positions[0].TraderAddress = traderAddr.String()
			tc.Positions[0].TraderAddress = traderAddr.String()

			ctx, app, queryServer := initAppVpools(
				t,
				/* quoteReserve */ sdk.NewDec(100_000),
				/* baseReserve */ sdk.NewDec(100_000),
			)

			t.Log("initialize position")
			for _, position := range tc.Positions {
				currentPosition := position
				currentPosition.TraderAddress = traderAddr.String()
				setPosition(app.PerpKeeper, ctx, *currentPosition)
			}

			t.Log("query position")
			ctx = ctx.WithBlockTime(ctx.BlockTime().Add(time.Second))
			resp, err := queryServer.QueryPositions(
				sdk.WrapSDKContext(ctx),
				&types.QueryPositionsRequest{
					Trader: traderAddr.String(),
				},
			)
			require.NoError(t, err)

			t.Log("assert response")
			assert.Equal(t, len(tc.Positions), len(resp.Positions))
		})
	}
}

func TestQueryCumulativePremiumFraction(t *testing.T) {
	tests := []struct {
		name                string
		initialPairMetadata *types.PairMetadata

		query *types.QueryCumulativePremiumFractionRequest

		expectErr            bool
		expectedLatestCPF    sdk.Dec
		expectedEstimatedCPF sdk.Dec
	}{
		{
			name: "empty string pair",
			initialPairMetadata: &types.PairMetadata{
				Pair:                            common.Pair_BTC_NUSD,
				LatestCumulativePremiumFraction: sdk.ZeroDec(),
			},
			query: &types.QueryCumulativePremiumFractionRequest{
				Pair: "",
			},
			expectErr: true,
		},
		{
			name: "pair metadata not found",
			initialPairMetadata: &types.PairMetadata{
				Pair:                            common.Pair_BTC_NUSD,
				LatestCumulativePremiumFraction: sdk.ZeroDec(),
			},
			query: &types.QueryCumulativePremiumFractionRequest{
				Pair: "foo:bar",
			},
			expectErr: true,
		},
		{
			name: "returns single funding payment",
			initialPairMetadata: &types.PairMetadata{
				Pair:                            common.Pair_BTC_NUSD,
				LatestCumulativePremiumFraction: sdk.ZeroDec(),
			},
			query: &types.QueryCumulativePremiumFractionRequest{
				Pair: common.Pair_BTC_NUSD.String(),
			},
			expectErr:            false,
			expectedLatestCPF:    sdk.ZeroDec(),
			expectedEstimatedCPF: sdk.NewDec(10), // (481 - 1) / 48
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Log("initialize app and keeper")
			ctx, app, queryServer := initAppVpools(t, sdk.NewDec(481_000), sdk.NewDec(1_000))

			t.Log("set index price")
			oracle := testutil.AccAddress()
			app.PricefeedKeeper.WhitelistOracles(ctx, []sdk.AccAddress{oracle})
			require.NoError(t, app.PricefeedKeeper.PostRawPrice(ctx, oracle, common.Pair_BTC_NUSD.String(), sdk.OneDec(), time.Now().Add(time.Hour)))
			require.NoError(t, app.PricefeedKeeper.GatherRawPrices(ctx, common.DenomBTC, common.DenomNUSD))

			t.Log("advance block time to realize index price")
			ctx = ctx.WithBlockTime(ctx.BlockTime().Add(time.Second))

			t.Log("query cumulative premium fraction")
			resp, err := queryServer.CumulativePremiumFraction(sdk.WrapSDKContext(ctx), tc.query)

			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.EqualValues(t, tc.expectedLatestCPF, resp.CumulativePremiumFraction)
				assert.EqualValues(t, tc.expectedEstimatedCPF, resp.EstimatedNextCumulativePremiumFraction)
			}
		})
	}
}

func TestQueryMetrics(t *testing.T) {
	tests := []struct {
		name             string
		BaseAssetAmounts []sdk.Dec
		NetSize          sdk.Dec
	}{
		{
			name:             "zero net_size",
			BaseAssetAmounts: []sdk.Dec{},
			NetSize:          sdk.ZeroDec(),
		},
		{
			name: "positice net_size",
			BaseAssetAmounts: []sdk.Dec{
				sdk.NewDec(10),
				sdk.NewDec(20),
				sdk.NewDec(30),
			},
			NetSize: sdk.NewDec(60),
		},
		{
			name: "negative net_size",
			BaseAssetAmounts: []sdk.Dec{
				sdk.NewDec(10),
				sdk.NewDec(-50),
				sdk.NewDec(30),
			},
			NetSize: sdk.NewDec(-10),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx, app, queryServer := initAppVpools(
				t,
				/* quoteReserve */ sdk.NewDec(100_000),
				/* baseReserve */ sdk.NewDec(100_000),
			)

			t.Log("call OnSwapEnd hook")
			for _, baseAssetAmount := range tc.BaseAssetAmounts {
				app.PerpKeeper.OnSwapEnd(ctx, common.Pair_BTC_NUSD, sdk.ZeroDec(), baseAssetAmount)
			}

			t.Log("query metrics")
			resp, err := queryServer.Metrics(
				sdk.WrapSDKContext(ctx),
				&types.QueryMetricsRequest{
					Pair: common.Pair_BTC_NUSD.String(),
				},
			)
			require.NoError(t, err)

			t.Log("assert response")
			assert.Equal(t, tc.NetSize, resp.Metrics.NetSize)
		})
	}
}
