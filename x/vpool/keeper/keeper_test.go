package keeper

import (
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/testutil/mock"
	"github.com/NibiruChain/nibiru/x/vpool/types"
)

func TestSwapQuoteForBase(t *testing.T) {
	tests := []struct {
		name                      string
		pair                      common.AssetPair
		direction                 types.Direction
		quoteAmount               sdk.Dec
		baseLimit                 sdk.Dec
		skipFluctuationLimitCheck bool

		expectedQuoteReserve sdk.Dec
		expectedBaseReserve  sdk.Dec
		expectedBaseAmount   sdk.Dec
		expectedErr          error
	}{
		{
			name:                      "quote amount == 0",
			pair:                      BTCNusdPair,
			direction:                 types.Direction_ADD_TO_POOL,
			quoteAmount:               sdk.NewDec(0),
			baseLimit:                 sdk.NewDec(10),
			skipFluctuationLimitCheck: false,

			expectedQuoteReserve: sdk.NewDec(10_000_000),
			expectedBaseReserve:  sdk.NewDec(5_000_000),
			expectedBaseAmount:   sdk.ZeroDec(),
		},
		{
			name:                      "normal swap add",
			pair:                      BTCNusdPair,
			direction:                 types.Direction_ADD_TO_POOL,
			quoteAmount:               sdk.NewDec(100_000),
			baseLimit:                 sdk.NewDec(49504),
			skipFluctuationLimitCheck: false,

			expectedQuoteReserve: sdk.NewDec(10_100_000),
			expectedBaseReserve:  sdk.MustNewDecFromStr("4950495.049504950495049505"),
			expectedBaseAmount:   sdk.MustNewDecFromStr("49504.950495049504950495"),
		},
		{
			name:                      "normal swap remove",
			pair:                      BTCNusdPair,
			direction:                 types.Direction_REMOVE_FROM_POOL,
			quoteAmount:               sdk.NewDec(100_000),
			baseLimit:                 sdk.NewDec(50506),
			skipFluctuationLimitCheck: false,

			expectedQuoteReserve: sdk.NewDec(9_900_000),
			expectedBaseReserve:  sdk.MustNewDecFromStr("5050505.050505050505050505"),
			expectedBaseAmount:   sdk.MustNewDecFromStr("50505.050505050505050505"),
		},
		{
			name:                      "pair not supported",
			pair:                      common.AssetPair{Token0: "abc", Token1: "xyz"},
			direction:                 types.Direction_ADD_TO_POOL,
			quoteAmount:               sdk.NewDec(10),
			baseLimit:                 sdk.NewDec(10),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrPairNotSupported,
		},
		{
			name:                      "base amount less than base limit in Long",
			pair:                      BTCNusdPair,
			direction:                 types.Direction_ADD_TO_POOL,
			quoteAmount:               sdk.NewDec(500_000),
			baseLimit:                 sdk.NewDec(454_500),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrAssetFailsUserLimit,
		},
		{
			name:                      "base amount more than base limit in Short",
			pair:                      BTCNusdPair,
			direction:                 types.Direction_REMOVE_FROM_POOL,
			quoteAmount:               sdk.NewDec(1_000_000),
			baseLimit:                 sdk.NewDec(454_500),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrAssetFailsUserLimit,
		},
		{
			name:                      "quote input bigger than trade limit ratio",
			pair:                      BTCNusdPair,
			direction:                 types.Direction_REMOVE_FROM_POOL,
			quoteAmount:               sdk.NewDec(10_000_000),
			baseLimit:                 sdk.NewDec(10),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrOverTradingLimit,
		},
		{
			name:                      "over fluctuation limit fails on add",
			pair:                      BTCNusdPair,
			direction:                 types.Direction_ADD_TO_POOL,
			quoteAmount:               sdk.NewDec(1_000_000),
			baseLimit:                 sdk.NewDec(454_544),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrOverFluctuationLimit,
		},
		{
			name:                      "over fluctuation limit fails on remove",
			pair:                      BTCNusdPair,
			direction:                 types.Direction_REMOVE_FROM_POOL,
			quoteAmount:               sdk.NewDec(1_000_000),
			baseLimit:                 sdk.NewDec(555_556),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrOverFluctuationLimit,
		},
		{
			name:                      "over fluctuation limit allowed on add",
			pair:                      BTCNusdPair,
			direction:                 types.Direction_ADD_TO_POOL,
			quoteAmount:               sdk.NewDec(1_000_000),
			baseLimit:                 sdk.NewDec(454_544),
			skipFluctuationLimitCheck: true,

			expectedQuoteReserve: sdk.NewDec(11_000_000),
			expectedBaseReserve:  sdk.MustNewDecFromStr("4545454.545454545454545455"),
			expectedBaseAmount:   sdk.MustNewDecFromStr("454545.454545454545454545"),
		},
		{
			name:                      "over fluctuation limit allowed on remove",
			pair:                      BTCNusdPair,
			direction:                 types.Direction_REMOVE_FROM_POOL,
			quoteAmount:               sdk.NewDec(1_000_000),
			baseLimit:                 sdk.NewDec(555_556),
			skipFluctuationLimitCheck: true,

			expectedQuoteReserve: sdk.NewDec(9_000_000),
			expectedBaseReserve:  sdk.MustNewDecFromStr("5555555.555555555555555556"),
			expectedBaseAmount:   sdk.MustNewDecFromStr("555555.555555555555555556"),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			vpoolKeeper, ctx := VpoolKeeper(t,
				mock.NewMockPricefeedKeeper(gomock.NewController(t)),
			)

			vpoolKeeper.CreatePool(
				ctx,
				BTCNusdPair,
				/* tradeLimitRatio */ sdk.MustNewDecFromStr("0.9"),
				/* quoteAssetReserve */ sdk.NewDec(10_000_000), // 10 tokens
				/* baseAssetReserve */ sdk.NewDec(5_000_000), // 5 tokens
				/* fluctuationLimitRatio */ sdk.MustNewDecFromStr("0.1"),
				/* maxOracleSpreadRatio */ sdk.MustNewDecFromStr("0.1"),
				/* maintenanceMarginRatio */ sdk.MustNewDecFromStr("0.0625"),
			)

			baseAmt, err := vpoolKeeper.SwapQuoteForBase(
				ctx,
				tc.pair,
				tc.direction,
				tc.quoteAmount,
				tc.baseLimit,
				tc.skipFluctuationLimitCheck,
			)

			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
				assert.EqualValuesf(t, tc.expectedBaseAmount, baseAmt, "base amount mismatch")

				t.Log("assert vpool")
				pool, err := vpoolKeeper.getPool(ctx, BTCNusdPair)
				require.NoError(t, err)
				assert.EqualValuesf(t, tc.expectedQuoteReserve, pool.QuoteAssetReserve, "pool quote asset reserve mismatch")
				assert.EqualValuesf(t, tc.expectedBaseReserve, pool.BaseAssetReserve, "pool base asset reserve mismatch")

				t.Log("assert snapshot")
				snapshot, _, err := vpoolKeeper.getLatestReserveSnapshot(ctx, BTCNusdPair)
				require.NoError(t, err)
				assert.EqualValuesf(t, tc.expectedQuoteReserve, snapshot.QuoteAssetReserve, "snapshot quote asset reserve mismatch")
				assert.EqualValuesf(t, tc.expectedBaseReserve, snapshot.BaseAssetReserve, "snapshot base asset reserve mismatch")
			}
		})
	}
}

func TestSwapBaseForQuote(t *testing.T) {
	tests := []struct {
		name                      string
		pair                      common.AssetPair
		direction                 types.Direction
		baseAmt                   sdk.Dec
		quoteLimit                sdk.Dec
		skipFluctuationLimitCheck bool

		expectedQuoteReserve     sdk.Dec
		expectedBaseReserve      sdk.Dec
		expectedQuoteAssetAmount sdk.Dec
		expectedErr              error
	}{
		{
			name:                      "zero base asset swap",
			pair:                      BTCNusdPair,
			direction:                 types.Direction_ADD_TO_POOL,
			baseAmt:                   sdk.ZeroDec(),
			quoteLimit:                sdk.ZeroDec(),
			skipFluctuationLimitCheck: false,

			expectedQuoteReserve:     sdk.NewDec(10_000_000),
			expectedBaseReserve:      sdk.NewDec(5_000_000),
			expectedQuoteAssetAmount: sdk.ZeroDec(),
		},
		{
			name:                      "add base asset swap",
			pair:                      BTCNusdPair,
			direction:                 types.Direction_ADD_TO_POOL,
			baseAmt:                   sdk.NewDec(100_000),
			quoteLimit:                sdk.NewDec(196078),
			skipFluctuationLimitCheck: false,

			expectedQuoteReserve:     sdk.MustNewDecFromStr("9803921.568627450980392157"),
			expectedBaseReserve:      sdk.NewDec(5_100_000),
			expectedQuoteAssetAmount: sdk.MustNewDecFromStr("196078.431372549019607843"),
		},
		{
			name:                      "remove base asset",
			pair:                      BTCNusdPair,
			direction:                 types.Direction_REMOVE_FROM_POOL,
			baseAmt:                   sdk.NewDec(100_000),
			quoteLimit:                sdk.NewDec(204_082),
			skipFluctuationLimitCheck: false,

			expectedQuoteReserve:     sdk.MustNewDecFromStr("10204081.632653061224489796"),
			expectedBaseReserve:      sdk.NewDec(4_900_000),
			expectedQuoteAssetAmount: sdk.MustNewDecFromStr("204081.632653061224489796"),
		},
		{
			name:                      "pair not supported",
			pair:                      common.AssetPair{Token0: "abc", Token1: "xyz"},
			direction:                 types.Direction_ADD_TO_POOL,
			baseAmt:                   sdk.NewDec(10),
			quoteLimit:                sdk.NewDec(10),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrPairNotSupported,
		},
		{
			name:                      "quote amount less than quote limit in Long",
			pair:                      BTCNusdPair,
			direction:                 types.Direction_ADD_TO_POOL,
			baseAmt:                   sdk.NewDec(100_000),
			quoteLimit:                sdk.NewDec(196079),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrAssetFailsUserLimit,
		},
		{
			name:                      "quote amount more than quote limit in Short",
			pair:                      BTCNusdPair,
			direction:                 types.Direction_REMOVE_FROM_POOL,
			baseAmt:                   sdk.NewDec(100_000),
			quoteLimit:                sdk.NewDec(204_081),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrAssetFailsUserLimit,
		},
		{
			name:                      "base input bigger than trade limit ratio",
			pair:                      BTCNusdPair,
			direction:                 types.Direction_REMOVE_FROM_POOL,
			baseAmt:                   sdk.NewDec(4_500_001),
			quoteLimit:                sdk.NewDec(10),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrOverTradingLimit,
		},
		{
			name:                      "over fluctuation limit fails on add",
			pair:                      BTCNusdPair,
			direction:                 types.Direction_ADD_TO_POOL,
			baseAmt:                   sdk.NewDec(1_000_000),
			quoteLimit:                sdk.NewDec(1_666_666),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrOverFluctuationLimit,
		},
		{
			name:                      "over fluctuation limit fails on remove",
			pair:                      BTCNusdPair,
			direction:                 types.Direction_REMOVE_FROM_POOL,
			baseAmt:                   sdk.NewDec(1_000_000),
			quoteLimit:                sdk.NewDec(2_500_001),
			skipFluctuationLimitCheck: false,

			expectedErr: types.ErrOverFluctuationLimit,
		},
		{
			name:                      "over fluctuation limit allowed on add",
			pair:                      BTCNusdPair,
			direction:                 types.Direction_ADD_TO_POOL,
			baseAmt:                   sdk.NewDec(1_000_000),
			quoteLimit:                sdk.NewDec(1_666_666),
			skipFluctuationLimitCheck: true,

			expectedQuoteReserve:     sdk.MustNewDecFromStr("8333333.333333333333333333"),
			expectedBaseReserve:      sdk.NewDec(6_000_000),
			expectedQuoteAssetAmount: sdk.MustNewDecFromStr("1666666.666666666666666667"),
		},
		{
			name:                      "over fluctuation limit allowed on remove",
			pair:                      BTCNusdPair,
			direction:                 types.Direction_REMOVE_FROM_POOL,
			baseAmt:                   sdk.NewDec(1_000_000),
			quoteLimit:                sdk.NewDec(2_500_001),
			skipFluctuationLimitCheck: true,

			expectedQuoteReserve:     sdk.NewDec(12_500_000),
			expectedBaseReserve:      sdk.NewDec(4_000_000),
			expectedQuoteAssetAmount: sdk.NewDec(2_500_000),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			vpoolKeeper, ctx := VpoolKeeper(t,
				mock.NewMockPricefeedKeeper(gomock.NewController(t)),
			)

			vpoolKeeper.CreatePool(
				ctx,
				BTCNusdPair,
				/* tradeLimitRatio */ sdk.MustNewDecFromStr("0.9"),
				/* quoteAssetReserve */ sdk.NewDec(10_000_000), // 10 tokens
				/* baseAssetReserve */ sdk.NewDec(5_000_000), // 5 tokens
				/* fluctuationLimitRatio */ sdk.MustNewDecFromStr("0.1"),
				/* maxOracleSpreadRatio */ sdk.MustNewDecFromStr("0.1"),
				/* maintenanceMarginRatio */ sdk.MustNewDecFromStr("0.0625"),
			)

			quoteAssetAmount, err := vpoolKeeper.SwapBaseForQuote(
				ctx,
				tc.pair,
				tc.direction,
				tc.baseAmt,
				tc.quoteLimit,
				tc.skipFluctuationLimitCheck,
			)

			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
				assert.EqualValuesf(t, tc.expectedQuoteAssetAmount, quoteAssetAmount,
					"expected %s; got %s", tc.expectedQuoteAssetAmount.String(), quoteAssetAmount.String())

				t.Log("assert pool")
				pool, err := vpoolKeeper.getPool(ctx, BTCNusdPair)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedQuoteReserve, pool.QuoteAssetReserve)
				assert.Equal(t, tc.expectedBaseReserve, pool.BaseAssetReserve)

				t.Log("assert snapshot")
				snapshot, _, err := vpoolKeeper.getLatestReserveSnapshot(ctx, BTCNusdPair)
				require.NoError(t, err)
				assert.EqualValues(t, tc.expectedQuoteReserve, snapshot.QuoteAssetReserve)
				assert.EqualValues(t, tc.expectedBaseReserve, snapshot.BaseAssetReserve)
			}
		})
	}
}

func TestGetVpools(t *testing.T) {
	vpoolKeeper, ctx := VpoolKeeper(t,
		mock.NewMockPricefeedKeeper(gomock.NewController(t)),
	)

	vpoolKeeper.CreatePool(
		ctx,
		BTCNusdPair,
		sdk.OneDec(),
		sdk.NewDec(10_000_000),
		sdk.NewDec(5_000_000),
		sdk.OneDec(),
		sdk.OneDec(),
		sdk.MustNewDecFromStr("0.0625"),
	)
	vpoolKeeper.CreatePool(
		ctx,
		ETHNusdPair,
		sdk.OneDec(),
		sdk.NewDec(5_000_000),
		sdk.NewDec(10_000_000),
		sdk.OneDec(),
		sdk.OneDec(),
		sdk.MustNewDecFromStr("0.0625"),
	)

	pools := vpoolKeeper.GetAllPools(ctx)

	require.EqualValues(t, 2, len(pools))

	require.EqualValues(t, *pools[0], types.Pool{
		Pair:                   BTCNusdPair,
		BaseAssetReserve:       sdk.NewDec(5_000_000),
		QuoteAssetReserve:      sdk.NewDec(10_000_000),
		TradeLimitRatio:        sdk.OneDec(),
		FluctuationLimitRatio:  sdk.OneDec(),
		MaxOracleSpreadRatio:   sdk.OneDec(),
		MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
	})
	require.EqualValues(t, *pools[1], types.Pool{
		Pair:                   ETHNusdPair,
		BaseAssetReserve:       sdk.NewDec(10_000_000),
		QuoteAssetReserve:      sdk.NewDec(5_000_000),
		TradeLimitRatio:        sdk.OneDec(),
		FluctuationLimitRatio:  sdk.OneDec(),
		MaxOracleSpreadRatio:   sdk.OneDec(),
		MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
	})
}

func TestIsOverFluctuationLimit(t *testing.T) {
	tests := []struct {
		name     string
		pool     types.Pool
		snapshot types.ReserveSnapshot

		isOverLimit bool
	}{
		{
			name: "zero fluctuation limit ratio",
			pool: types.Pool{
				Pair:                   common.PairBTCStable,
				QuoteAssetReserve:      sdk.OneDec(),
				BaseAssetReserve:       sdk.OneDec(),
				FluctuationLimitRatio:  sdk.ZeroDec(),
				TradeLimitRatio:        sdk.OneDec(),
				MaxOracleSpreadRatio:   sdk.OneDec(),
				MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			},
			snapshot: types.ReserveSnapshot{
				QuoteAssetReserve: sdk.NewDec(1000),
				BaseAssetReserve:  sdk.OneDec(),
				TimestampMs:       0,
				BlockNumber:       0,
			},
			isOverLimit: false,
		},
		{
			name: "lower limit of fluctuation limit",
			pool: types.Pool{
				Pair:                   common.PairBTCStable,
				QuoteAssetReserve:      sdk.NewDec(999),
				BaseAssetReserve:       sdk.OneDec(),
				FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.001"),
				TradeLimitRatio:        sdk.OneDec(),
				MaxOracleSpreadRatio:   sdk.OneDec(),
				MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			},
			snapshot: types.ReserveSnapshot{
				QuoteAssetReserve: sdk.NewDec(1000),
				BaseAssetReserve:  sdk.OneDec(),
				TimestampMs:       0,
				BlockNumber:       0,
			},
			isOverLimit: false,
		},
		{
			name: "upper limit of fluctuation limit",
			pool: types.Pool{
				Pair:                   common.PairBTCStable,
				QuoteAssetReserve:      sdk.NewDec(1001),
				BaseAssetReserve:       sdk.OneDec(),
				FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.001"),
				TradeLimitRatio:        sdk.OneDec(),
				MaxOracleSpreadRatio:   sdk.OneDec(),
				MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			},
			snapshot: types.ReserveSnapshot{
				QuoteAssetReserve: sdk.NewDec(1000),
				BaseAssetReserve:  sdk.OneDec(),
				TimestampMs:       0,
				BlockNumber:       0,
			},
			isOverLimit: false,
		},
		{
			name: "under fluctuation limit",
			pool: types.Pool{
				Pair:                   common.PairBTCStable,
				QuoteAssetReserve:      sdk.NewDec(998),
				BaseAssetReserve:       sdk.OneDec(),
				FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.001"),
				TradeLimitRatio:        sdk.OneDec(),
				MaxOracleSpreadRatio:   sdk.OneDec(),
				MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			},
			snapshot: types.ReserveSnapshot{
				QuoteAssetReserve: sdk.NewDec(1000),
				BaseAssetReserve:  sdk.OneDec(),
				TimestampMs:       0,
				BlockNumber:       0,
			},
			isOverLimit: true,
		},
		{
			name: "over fluctuation limit",
			pool: types.Pool{
				Pair:                   common.PairBTCStable,
				QuoteAssetReserve:      sdk.NewDec(1002),
				BaseAssetReserve:       sdk.OneDec(),
				FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.001"),
				TradeLimitRatio:        sdk.OneDec(),
				MaxOracleSpreadRatio:   sdk.OneDec(),
				MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			},
			snapshot: types.ReserveSnapshot{
				QuoteAssetReserve: sdk.NewDec(1000),
				BaseAssetReserve:  sdk.OneDec(),
				TimestampMs:       0,
				BlockNumber:       0,
			},
			isOverLimit: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			assert.EqualValues(t, tc.isOverLimit, isOverFluctuationLimit(&tc.pool, tc.snapshot))
		})
	}
}

func TestCheckFluctuationLimitRatio(t *testing.T) {
	tests := []struct {
		name           string
		pool           *types.Pool
		prevSnapshot   *types.ReserveSnapshot
		latestSnapshot *types.ReserveSnapshot
		ctxBlockHeight int64

		expectedErr error
	}{
		{
			name: "uses latest snapshot - does not result in error",
			pool: &types.Pool{
				Pair:                   common.PairBTCStable,
				QuoteAssetReserve:      sdk.NewDec(1002),
				BaseAssetReserve:       sdk.OneDec(),
				FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.001"),
				TradeLimitRatio:        sdk.OneDec(),
				MaxOracleSpreadRatio:   sdk.OneDec(),
				MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			},
			prevSnapshot: &types.ReserveSnapshot{
				QuoteAssetReserve: sdk.NewDec(1000),
				BaseAssetReserve:  sdk.OneDec(),
				TimestampMs:       0,
				BlockNumber:       0,
			},
			latestSnapshot: &types.ReserveSnapshot{
				QuoteAssetReserve: sdk.NewDec(1002),
				BaseAssetReserve:  sdk.OneDec(),
				TimestampMs:       1,
				BlockNumber:       1,
			},
			ctxBlockHeight: 2,
			expectedErr:    nil,
		},
		{
			name: "uses previous snapshot snapshot - results in error",
			pool: &types.Pool{
				Pair:                   common.PairBTCStable,
				QuoteAssetReserve:      sdk.NewDec(1002),
				BaseAssetReserve:       sdk.OneDec(),
				FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.001"),
				TradeLimitRatio:        sdk.OneDec(),
				MaxOracleSpreadRatio:   sdk.OneDec(),
				MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			},
			prevSnapshot: &types.ReserveSnapshot{
				QuoteAssetReserve: sdk.NewDec(1000),
				BaseAssetReserve:  sdk.OneDec(),
				TimestampMs:       0,
				BlockNumber:       0,
			},
			latestSnapshot: &types.ReserveSnapshot{
				QuoteAssetReserve: sdk.NewDec(1002),
				BaseAssetReserve:  sdk.OneDec(),
				TimestampMs:       1,
				BlockNumber:       1,
			},
			ctxBlockHeight: 1,
			expectedErr:    types.ErrOverFluctuationLimit,
		},
		{
			name: "only one snapshot - no error",
			pool: &types.Pool{
				Pair:                   common.PairBTCStable,
				QuoteAssetReserve:      sdk.NewDec(1000),
				BaseAssetReserve:       sdk.OneDec(),
				FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.001"),
				TradeLimitRatio:        sdk.OneDec(),
				MaxOracleSpreadRatio:   sdk.OneDec(),
				MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			},
			prevSnapshot: &types.ReserveSnapshot{
				QuoteAssetReserve: sdk.NewDec(1000),
				BaseAssetReserve:  sdk.OneDec(),
				TimestampMs:       0,
				BlockNumber:       0,
			},
			latestSnapshot: nil,
			ctxBlockHeight: 0,
			expectedErr:    nil,
		},
		{
			name: "zero fluctuation limit - no error",
			pool: &types.Pool{
				Pair:                   common.PairBTCStable,
				QuoteAssetReserve:      sdk.NewDec(2000),
				BaseAssetReserve:       sdk.OneDec(),
				FluctuationLimitRatio:  sdk.ZeroDec(),
				TradeLimitRatio:        sdk.OneDec(),
				MaxOracleSpreadRatio:   sdk.OneDec(),
				MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			},
			prevSnapshot: &types.ReserveSnapshot{
				QuoteAssetReserve: sdk.NewDec(1000),
				BaseAssetReserve:  sdk.OneDec(),
				TimestampMs:       0,
				BlockNumber:       0,
			},
			latestSnapshot: &types.ReserveSnapshot{
				QuoteAssetReserve: sdk.NewDec(1002),
				BaseAssetReserve:  sdk.OneDec(),
				TimestampMs:       1,
				BlockNumber:       1,
			},
			ctxBlockHeight: 2,
			expectedErr:    nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			vpoolKeeper, ctx := VpoolKeeper(t,
				mock.NewMockPricefeedKeeper(gomock.NewController(t)),
			)

			vpoolKeeper.savePool(ctx, tc.pool)

			t.Log("save snapshot 0")
			ctx = ctx.WithBlockHeight(tc.prevSnapshot.BlockNumber).WithBlockTime(time.UnixMilli(tc.prevSnapshot.TimestampMs))
			vpoolKeeper.saveSnapshot(ctx, common.PairBTCStable, 0, tc.prevSnapshot.QuoteAssetReserve, tc.prevSnapshot.BaseAssetReserve)
			vpoolKeeper.saveSnapshotCounter(ctx, common.PairBTCStable, 0)

			if tc.latestSnapshot != nil {
				t.Log("save snapshot 1")
				ctx = ctx.WithBlockHeight(tc.latestSnapshot.BlockNumber).WithBlockTime(time.UnixMilli(tc.latestSnapshot.TimestampMs))
				vpoolKeeper.saveSnapshot(ctx, common.PairBTCStable, 1, tc.latestSnapshot.QuoteAssetReserve, tc.latestSnapshot.BaseAssetReserve)
				vpoolKeeper.saveSnapshotCounter(ctx, common.PairBTCStable, 1)
			}

			t.Log("check fluctuation limit")
			ctx = ctx.WithBlockHeight(tc.ctxBlockHeight)
			err := vpoolKeeper.checkFluctuationLimitRatio(ctx, tc.pool)

			t.Log("check error if any")
			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGetMaintenanceMarginRatio(t *testing.T) {
	tests := []struct {
		name     string
		pool     *types.Pool
		snapshot types.ReserveSnapshot

		expectedMaintenanceMarginRatio sdk.Dec
	}{
		{
			name: "zero fluctuation limit ratio",
			pool: &types.Pool{
				Pair:                   common.PairBTCStable,
				QuoteAssetReserve:      sdk.OneDec(),
				BaseAssetReserve:       sdk.OneDec(),
				FluctuationLimitRatio:  sdk.ZeroDec(),
				TradeLimitRatio:        sdk.OneDec(),
				MaxOracleSpreadRatio:   sdk.OneDec(),
				MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.42"),
			},
			snapshot: types.ReserveSnapshot{
				QuoteAssetReserve: sdk.NewDec(1000),
				BaseAssetReserve:  sdk.OneDec(),
				TimestampMs:       0,
				BlockNumber:       0,
			},
			expectedMaintenanceMarginRatio: sdk.MustNewDecFromStr("0.42"),
		},
		{
			name: "zero fluctuation limit ratio",
			pool: &types.Pool{
				Pair:                   common.PairBTCStable,
				QuoteAssetReserve:      sdk.OneDec(),
				BaseAssetReserve:       sdk.OneDec(),
				FluctuationLimitRatio:  sdk.ZeroDec(),
				TradeLimitRatio:        sdk.OneDec(),
				MaxOracleSpreadRatio:   sdk.OneDec(),
				MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.4242"),
			},
			snapshot: types.ReserveSnapshot{
				QuoteAssetReserve: sdk.NewDec(1000),
				BaseAssetReserve:  sdk.OneDec(),
				TimestampMs:       0,
				BlockNumber:       0,
			},
			expectedMaintenanceMarginRatio: sdk.MustNewDecFromStr("0.4242"),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			vpoolKeeper, ctx := VpoolKeeper(t,
				mock.NewMockPricefeedKeeper(gomock.NewController(t)),
			)
			vpoolKeeper.savePool(ctx, tc.pool)

			assert.EqualValues(t, tc.expectedMaintenanceMarginRatio, vpoolKeeper.GetMaintenanceMarginRatio(ctx, common.PairBTCStable))
		})
	}
}
