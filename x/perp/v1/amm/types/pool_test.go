package types

import (
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/common/asset"
	"github.com/NibiruChain/nibiru/x/common/denoms"
	"github.com/NibiruChain/nibiru/x/common/testutil"
)

func TestMarket_NewPool(t *testing.T) {
	pair := asset.Registry.Pair(denoms.BTC, denoms.NUSD)
	tests := []struct {
		name        string
		args        ArgsNewMarket
		shouldPanic bool
		sqrtDepth   sdk.Dec
	}{
		{name: "pass: normal",
			args: ArgsNewMarket{
				Pair:          pair,
				BaseReserves:  sdk.NewDec(10 * 10), // 10**2
				QuoteReserves: sdk.NewDec(15 * 15), // 15**2
			}, shouldPanic: false, sqrtDepth: sdk.NewDec(150), // 10 * 15
		},
		{name: "pass: zero reserves",
			args: ArgsNewMarket{
				Pair:          pair,
				BaseReserves:  sdk.NewDec(10),
				QuoteReserves: sdk.NewDec(0),
			}, shouldPanic: false, sqrtDepth: sdk.NewDec(0),
		},
		{name: "pass: custom config",
			args: ArgsNewMarket{
				Pair:          pair,
				BaseReserves:  sdk.NewDec(22 * 22), // 22**2
				QuoteReserves: sdk.NewDec(7 * 7),   // 7**2
				Config: &MarketConfig{
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.1"),
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.1"),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					MaintenanceMarginRatio: sdk.OneDec().Quo(sdk.NewDec(18)),
					MaxLeverage:            sdk.NewDec(12),
				},
			}, shouldPanic: false, sqrtDepth: sdk.NewDec(154), // 22 * 7
		},
		{name: "err: negative sqrt depth",
			args: ArgsNewMarket{
				Pair:          pair,
				BaseReserves:  sdk.NewDec(10),
				QuoteReserves: sdk.NewDec(-10),
			}, shouldPanic: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.shouldPanic {
				require.Panics(t, func() {
					_ = NewMarket(tc.args)
				})
			} else {
				market := NewMarket(tc.args)
				assert.EqualValues(t, tc.args.Pair, market.Pair)
				assert.EqualValues(t, tc.args.BaseReserves, market.BaseReserve)
				assert.EqualValues(t, tc.args.QuoteReserves, market.QuoteReserve)

				sqrtDepth, err := common.SqrtDec(tc.args.BaseReserves.Mul(tc.args.QuoteReserves))
				assert.NoError(t, err)
				assert.EqualValues(t, sqrtDepth, market.SqrtDepth)

				var config MarketConfig
				if tc.args.Config == nil {
					config = *DefaultMarketConfig()
				} else {
					config = *tc.args.Config
				}
				assert.EqualValues(t, config, market.Config)
			}
		})
	}
}

func TestPoolHasEnoughQuoteReserve(t *testing.T) {
	pair := asset.MustNewPair("BTC:NUSD")

	pool := &Market{
		Pair:         pair,
		QuoteReserve: sdk.NewDec(10 * common.TO_MICRO),
		BaseReserve:  sdk.NewDec(10 * common.TO_MICRO),
		SqrtDepth:    common.MustSqrtDec(sdk.NewDec(10 * 10 * common.TO_MICRO)),
		Config: MarketConfig{
			FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.1"),
			MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.1"),
			MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			MaxLeverage:            sdk.NewDec(15),
			TradeLimitRatio:        sdk.MustNewDecFromStr("0.9"), // 0.9
		},
	}

	// less than max ratio
	require.True(t, pool.HasEnoughQuoteReserve(sdk.NewDec(8*common.TO_MICRO)))

	// equal to ratio limit
	require.True(t, pool.HasEnoughQuoteReserve(sdk.NewDec(9*common.TO_MICRO)))

	// more than ratio limit
	require.False(t, pool.HasEnoughQuoteReserve(sdk.NewDec(9_000_001)))
}

func TestSetMarginRatioAndLeverage(t *testing.T) {
	pair := asset.MustNewPair("BTC:NUSD")

	pool := &Market{
		Pair:         pair,
		QuoteReserve: sdk.NewDec(10 * common.TO_MICRO),
		BaseReserve:  sdk.NewDec(10 * common.TO_MICRO),
		SqrtDepth:    common.MustSqrtDec(sdk.NewDec(10 * 10 * common.TO_MICRO)),
		Config: MarketConfig{
			FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.1"),
			MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.42"),
			MaxLeverage:            sdk.NewDec(15),
			MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.1"),
			TradeLimitRatio:        sdk.MustNewDecFromStr("0.9"), // 0.9
		},
	}

	require.Equal(t, sdk.MustNewDecFromStr("0.42"), pool.Config.MaintenanceMarginRatio)
	require.Equal(t, sdk.MustNewDecFromStr("15"), pool.Config.MaxLeverage)
}

func TestGetBaseAmountByQuoteAmount(t *testing.T) {
	pair := asset.MustNewPair("BTC:NUSD")

	tests := []struct {
		name               string
		baseReserve        sdk.Dec
		quoteReserve       sdk.Dec
		quoteIn            sdk.Dec
		expectedBaseAmount sdk.Dec
		expectedErr        error
	}{
		{
			name:               "quote amount zero",
			baseReserve:        sdk.NewDec(1000),
			quoteReserve:       sdk.NewDec(1000),
			quoteIn:            sdk.ZeroDec(),
			expectedBaseAmount: sdk.ZeroDec(),
		},
		{
			name:         "simple add quote to pool",
			baseReserve:  sdk.NewDec(1000),
			quoteReserve: sdk.NewDec(1000), // swapInvariant = 1000 * 1000
			quoteIn:      sdk.NewDec(500),  // quoteReserves = 1000 + 500
			// swapInvariant / quoteReserves - baseReserves = 333.33
			expectedBaseAmount: sdk.MustNewDecFromStr("333.333333333333333333"),
		},
		{
			name:         "simple remove quote from pool",
			baseReserve:  sdk.NewDec(1000),
			quoteReserve: sdk.NewDec(1000), // swapInvariant = 1000 * 1000
			quoteIn:      sdk.NewDec(-500), // quoteReserves = 1000 - 500
			// swapInvariant / quoteReserves - baseReserves = 1000
			expectedBaseAmount: sdk.NewDec(1000),
		},
		{
			name:         "too much quote removed results in error",
			baseReserve:  sdk.NewDec(1000),
			quoteReserve: sdk.NewDec(1000),  // swapInvariant = 1000 * 1000
			quoteIn:      sdk.NewDec(-1000), // quoteReserves = 1000 - 1000
			expectedErr:  ErrQuoteReserveAtZero,
		},
		{
			name:         "attempt to remove more than the quote reserves",
			baseReserve:  sdk.NewDec(1000),
			quoteReserve: sdk.NewDec(1000),
			quoteIn:      sdk.NewDec(-9999),
			expectedErr:  ErrQuoteReserveAtZero,
		},
		{
			name:               "add large amount to the quote reserves",
			baseReserve:        sdk.NewDec(1000),
			quoteReserve:       sdk.NewDec(1000),        // swapInvariant = 1000 * 1000
			quoteIn:            sdk.NewDec(999_555_999), // quoteReserves = 1000 + 999_555_999
			expectedBaseAmount: sdk.MustNewDecFromStr("999.998999556802663137"),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			pool := NewMarket(ArgsNewMarket{
				Pair:          pair,
				QuoteReserves: tc.quoteReserve,
				BaseReserves:  tc.baseReserve,
				PegMultiplier: sdk.OneDec(),
				TotalLong:     sdk.ZeroDec(),
				TotalShort:    sdk.ZeroDec(),
				Config: &MarketConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.1"),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.NewDec(15),
					MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.1"),
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.9"), // 0.9
				},
			})

			amount, err := pool.GetBaseAmountByQuoteAmount(tc.quoteIn)
			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr,
					"expected error: %w, got: %w", tc.expectedErr, err)
			} else {
				require.NoError(t, err)
				require.EqualValuesf(t, tc.expectedBaseAmount, amount,
					"expected quote: %s, got: %s", tc.expectedBaseAmount.String(), amount.String(),
				)
			}
		})
	}
}

func TestGetQuoteReserveByBase(t *testing.T) {
	pair := asset.MustNewPair("BTC:NUSD")

	tests := []struct {
		name                string
		baseReserve         sdk.Dec
		quoteReserve        sdk.Dec
		baseIn              sdk.Dec
		expectedQuoteAmount sdk.Dec
		expectedErr         error
	}{
		{
			name:                "base amount zero",
			baseReserve:         sdk.NewDec(1000),
			quoteReserve:        sdk.NewDec(1000),
			baseIn:              sdk.ZeroDec(),
			expectedQuoteAmount: sdk.ZeroDec(),
		},
		{
			name:                "simple add base to pool",
			baseReserve:         sdk.NewDec(1000),
			quoteReserve:        sdk.NewDec(1000),
			baseIn:              sdk.NewDec(500),
			expectedQuoteAmount: sdk.MustNewDecFromStr("333.333333333333333333"),
		},
		{
			name:                "simple remove base from pool",
			baseReserve:         sdk.NewDec(1000),
			quoteReserve:        sdk.NewDec(1000),
			baseIn:              sdk.NewDec(-500),
			expectedQuoteAmount: sdk.NewDec(1000),
		},
		{
			name:         "too much base removed results in error",
			baseReserve:  sdk.NewDec(1000),
			quoteReserve: sdk.NewDec(1000),
			baseIn:       sdk.NewDec(-1000),
			expectedErr:  ErrBaseReserveAtZero,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			pool := &Market{
				Pair:         pair,
				QuoteReserve: tc.quoteReserve,
				BaseReserve:  tc.baseReserve,
				SqrtDepth:    common.MustSqrtDec(tc.quoteReserve.Mul(tc.baseReserve)),
				TotalLong:    sdk.ZeroDec(),
				TotalShort:   sdk.ZeroDec(),
				Config: MarketConfig{
					FluctuationLimitRatio:  sdk.OneDec(),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.NewDec(15),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					TradeLimitRatio:        sdk.OneDec(),
				},
			}

			amount, err := pool.GetQuoteReserveByBase(tc.baseIn)
			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr,
					"expected error: %w, got: %w", tc.expectedErr, err)
			} else {
				require.NoError(t, err)
				require.EqualValuesf(t, tc.expectedQuoteAmount, amount,
					"expected quote: %s, got: %s", tc.expectedQuoteAmount.String(), amount.String(),
				)
			}
		})
	}
}

func TestIncreaseDecreaseReserves(t *testing.T) {
	pair := asset.MustNewPair("ATOM:NUSD")

	pool := NewMarket(ArgsNewMarket{
		Pair:          pair,
		QuoteReserves: sdk.NewDec(1 * common.TO_MICRO),
		BaseReserves:  sdk.NewDec(1 * common.TO_MICRO),
		TotalLong:     sdk.ZeroDec(),
		TotalShort:    sdk.ZeroDec(),
		PegMultiplier: sdk.OneDec(),
		Config: &MarketConfig{
			FluctuationLimitRatio:  sdk.OneDec(),
			MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			MaxLeverage:            sdk.NewDec(15),
			MaxOracleSpreadRatio:   sdk.OneDec(),
			TradeLimitRatio:        sdk.OneDec(),
		},
	})

	t.Log("decrease quote asset reserve")
	pool.AddToQuoteReserve(sdk.NewDec(-100))
	require.Equal(t, sdk.NewDec(999_900), pool.QuoteReserve)

	t.Log("increase quote asset reserve")
	pool.AddToQuoteReserve(sdk.NewDec(100))
	require.Equal(t, sdk.NewDec(1*common.TO_MICRO), pool.QuoteReserve)

	t.Log("decrease base asset reserve")
	pool.AddToBaseReserveAndTotalLongShort(sdk.NewDec(-100))
	require.Equal(t, sdk.NewDec(999_900), pool.BaseReserve)

	t.Log("increase base asset reserve")
	pool.AddToBaseReserveAndTotalLongShort(sdk.NewDec(100))
	require.Equal(t, sdk.NewDec(1*common.TO_MICRO), pool.BaseReserve)
}

func TestPool_Validate(t *testing.T) {
	type test struct {
		m         *Market
		expectErr bool
	}

	cases := map[string]test{
		"invalid pair": {
			m: &Market{
				Pair:          "",
				BaseReserve:   sdk.OneDec(),
				QuoteReserve:  sdk.OneDec(),
				PegMultiplier: sdk.OneDec(),
				SqrtDepth:     common.MustSqrtDec(sdk.NewDec(1)),
				Config: MarketConfig{
					TradeLimitRatio:        sdk.NewDec(-1),
					FluctuationLimitRatio:  sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					MaintenanceMarginRatio: sdk.OneDec(),
					MaxLeverage:            sdk.OneDec(),
				},
			},
			expectErr: true,
		},

		"invalid trade limit ratio < 0": {
			m: &Market{
				Pair:          asset.MustNewPair("btc:usd"),
				BaseReserve:   sdk.OneDec(),
				QuoteReserve:  sdk.OneDec(),
				PegMultiplier: sdk.OneDec(),
				SqrtDepth:     common.MustSqrtDec(sdk.NewDec(1)),
				Config: MarketConfig{
					TradeLimitRatio:        sdk.NewDec(-1),
					FluctuationLimitRatio:  sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					MaintenanceMarginRatio: sdk.OneDec(),
					MaxLeverage:            sdk.OneDec(),
				},
			},
			expectErr: true,
		},

		"invalid trade limit ratio > 1": {
			m: &Market{
				Pair:          asset.MustNewPair("btc:usd"),
				BaseReserve:   sdk.OneDec(),
				QuoteReserve:  sdk.OneDec(),
				PegMultiplier: sdk.OneDec(),
				SqrtDepth:     common.MustSqrtDec(sdk.NewDec(1)),
				Config: MarketConfig{
					TradeLimitRatio:        sdk.NewDec(2),
					FluctuationLimitRatio:  sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					MaintenanceMarginRatio: sdk.OneDec(),
					MaxLeverage:            sdk.OneDec(),
				},
			},
			expectErr: true,
		},

		"quote asset reserve 0": {
			m: &Market{
				Pair:          asset.MustNewPair("btc:usd"),
				BaseReserve:   sdk.NewDec(999),
				QuoteReserve:  sdk.ZeroDec(),
				PegMultiplier: sdk.OneDec(),
				SqrtDepth:     common.MustSqrtDec(sdk.NewDec(999)),
				Config: MarketConfig{
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
					FluctuationLimitRatio:  sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					MaintenanceMarginRatio: sdk.OneDec(),
					MaxLeverage:            sdk.OneDec(),
				},
			},
			expectErr: true,
		},

		"base asset reserve 0": {
			m: &Market{
				Pair:          asset.MustNewPair("btc:usd"),
				QuoteReserve:  sdk.NewDec(1 * common.TO_MICRO),
				PegMultiplier: sdk.OneDec(),
				BaseReserve:   sdk.ZeroDec(),
				SqrtDepth:     sdk.ZeroDec(),
				Config: MarketConfig{
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
					FluctuationLimitRatio:  sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					MaintenanceMarginRatio: sdk.OneDec(),
					MaxLeverage:            sdk.OneDec(),
				},
			},
			expectErr: true,
		},

		"fluctuation < 0": {
			m: &Market{
				Pair:          asset.MustNewPair("btc:usd"),
				QuoteReserve:  sdk.NewDec(1 * common.TO_MICRO),
				PegMultiplier: sdk.OneDec(),
				BaseReserve:   sdk.NewDec(1 * common.TO_MICRO),
				SqrtDepth:     common.MustSqrtDec(sdk.NewDec(1 * common.TO_MICRO * common.TO_MICRO)),
				Config: MarketConfig{
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
					FluctuationLimitRatio:  sdk.NewDec(-1),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					MaintenanceMarginRatio: sdk.OneDec(),
					MaxLeverage:            sdk.OneDec(),
				},
			},
			expectErr: true,
		},

		"fluctuation > 1": {
			m: &Market{
				Pair:          asset.MustNewPair("btc:usd"),
				QuoteReserve:  sdk.NewDec(1 * common.TO_MICRO),
				PegMultiplier: sdk.OneDec(),
				BaseReserve:   sdk.NewDec(1 * common.TO_MICRO),
				SqrtDepth:     common.MustSqrtDec(sdk.NewDec(1 * common.TO_MICRO * common.TO_MICRO)),
				Config: MarketConfig{
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
					FluctuationLimitRatio:  sdk.NewDec(2),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					MaintenanceMarginRatio: sdk.OneDec(),
					MaxLeverage:            sdk.OneDec(),
				},
			},
			expectErr: true,
		},

		"max oracle spread ratio < 0": {
			m: &Market{
				Pair:          asset.MustNewPair("btc:usd"),
				QuoteReserve:  sdk.NewDec(1 * common.TO_MICRO),
				PegMultiplier: sdk.OneDec(),
				BaseReserve:   sdk.NewDec(1 * common.TO_MICRO),
				SqrtDepth:     common.MustSqrtDec(sdk.NewDec(1 * common.TO_MICRO * common.TO_MICRO)),
				Config: MarketConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.10"),
					MaintenanceMarginRatio: sdk.OneDec(),
					MaxLeverage:            sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.NewDec(-1),
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
				},
			},
			expectErr: true,
		},

		"max oracle spread ratio > 1": {
			m: &Market{
				Pair:          asset.MustNewPair("btc:usd"),
				QuoteReserve:  sdk.NewDec(1 * common.TO_MICRO),
				PegMultiplier: sdk.OneDec(),
				BaseReserve:   sdk.NewDec(1 * common.TO_MICRO),
				SqrtDepth:     common.MustSqrtDec(sdk.NewDec(1 * common.TO_MICRO * common.TO_MICRO)),
				Config: MarketConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.10"),
					MaintenanceMarginRatio: sdk.OneDec(),
					MaxLeverage:            sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.NewDec(2),
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
				},
			},
			expectErr: true,
		},

		"maintenance ratio < 0": {
			m: &Market{
				Pair: asset.MustNewPair("btc:usd"),
				Config: MarketConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.10"),
					MaintenanceMarginRatio: sdk.NewDec(-1),
					MaxLeverage:            sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.10"),
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
				},
				QuoteReserve:  sdk.NewDec(1 * common.TO_MICRO),
				PegMultiplier: sdk.OneDec(),
				BaseReserve:   sdk.NewDec(1 * common.TO_MICRO),
				SqrtDepth:     common.MustSqrtDec(sdk.NewDec(1 * common.TO_MICRO * common.TO_MICRO)),
			},
			expectErr: true,
		},

		"maintenance ratio > 1": {
			m: &Market{
				Pair:          asset.MustNewPair("btc:usd"),
				QuoteReserve:  sdk.NewDec(1 * common.TO_MICRO),
				PegMultiplier: sdk.OneDec(),
				BaseReserve:   sdk.NewDec(1 * common.TO_MICRO),
				SqrtDepth:     common.MustSqrtDec(sdk.NewDec(1 * common.TO_MICRO * common.TO_MICRO)),
				Config: MarketConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.10"),
					MaintenanceMarginRatio: sdk.NewDec(2),
					MaxLeverage:            sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.10"),
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
				},
			},
			expectErr: true,
		},

		"max leverage < 0": {
			m: &Market{
				Pair:          asset.MustNewPair("btc:usd"),
				QuoteReserve:  sdk.NewDec(1 * common.TO_MICRO),
				PegMultiplier: sdk.OneDec(),
				BaseReserve:   sdk.NewDec(1 * common.TO_MICRO),
				SqrtDepth:     common.MustSqrtDec(sdk.NewDec(1 * common.TO_MICRO * common.TO_MICRO)),
				Config: MarketConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.10"),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.10"),
					MaxLeverage:            sdk.MustNewDecFromStr("-0.10"),
					MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.10"),
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
				},
			},
			expectErr: true,
		},

		"max leverage too high for maintenance margin ratio": {
			m: &Market{
				Pair:          asset.MustNewPair("btc:usd"),
				QuoteReserve:  sdk.NewDec(1 * common.TO_MICRO),
				PegMultiplier: sdk.OneDec(),
				BaseReserve:   sdk.NewDec(1 * common.TO_MICRO),
				SqrtDepth:     common.MustSqrtDec(sdk.NewDec(1 * common.TO_MICRO * common.TO_MICRO)),
				Config: MarketConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.10"),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.10"), // Equivalent to 10 leverage
					MaxLeverage:            sdk.MustNewDecFromStr("11"),
					MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.10"),
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
				},
			},
			expectErr: true,
		},

		"success": {
			m: &Market{
				Pair:          asset.MustNewPair("btc:usd"),
				QuoteReserve:  sdk.NewDec(1 * common.TO_MICRO),
				PegMultiplier: sdk.OneDec(),
				BaseReserve:   sdk.NewDec(1 * common.TO_MICRO),
				SqrtDepth:     common.MustSqrtDec(sdk.NewDec(1 * common.TO_MICRO * common.TO_MICRO)),
				Config: MarketConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.10"),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.MustNewDecFromStr("15"),
					MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.10"),
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
				},
			},
			expectErr: false,
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			err := tc.m.Validate()
			if err == nil && tc.expectErr {
				t.Fatal("error expected")
			} else if err != nil && !tc.expectErr {
				t.Fatal("unexpected error")
			}
		})
	}
}

func TestMarket_GetMarkPrice(t *testing.T) {
	tests := []struct {
		name          string
		pool          Market
		expectedValue sdk.Dec
	}{
		{
			"happy path",
			Market{
				Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				BaseReserve:   sdk.MustNewDecFromStr("10"),
				QuoteReserve:  sdk.MustNewDecFromStr("10000"),
				PegMultiplier: sdk.OneDec(),
			},
			sdk.MustNewDecFromStr("1000"),
		},
		{
			"nil base",
			Market{
				Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				BaseReserve:   sdk.Dec{},
				QuoteReserve:  sdk.MustNewDecFromStr("10000"),
				PegMultiplier: sdk.OneDec(),
			},
			sdk.ZeroDec(),
		},
		{
			"zero base",
			Market{
				Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				BaseReserve:   sdk.ZeroDec(),
				QuoteReserve:  sdk.MustNewDecFromStr("10000"),
				PegMultiplier: sdk.OneDec(),
			},
			sdk.ZeroDec(),
		},
		{
			"nil quote",
			Market{
				Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				BaseReserve:   sdk.MustNewDecFromStr("10"),
				QuoteReserve:  sdk.Dec{},
				PegMultiplier: sdk.OneDec(),
			},
			sdk.ZeroDec(),
		},
		{
			"zero quote",
			Market{
				Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				BaseReserve:   sdk.MustNewDecFromStr("10"),
				QuoteReserve:  sdk.ZeroDec(),
				PegMultiplier: sdk.OneDec(),
			},
			sdk.ZeroDec(),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require.True(t, tc.expectedValue.Equal(tc.pool.GetMarkPrice()))
		})
	}
}

func TestMarket_IsOverFluctuationLimit(t *testing.T) {
	tests := []struct {
		name string
		pool Market

		isOverLimit bool
	}{
		{
			name: "zero fluctuation limit ratio",
			pool: Market{
				Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				QuoteReserve:  sdk.OneDec(),
				BaseReserve:   sdk.OneDec(),
				PegMultiplier: sdk.OneDec(),
				Config: MarketConfig{
					FluctuationLimitRatio:  sdk.ZeroDec(),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.MustNewDecFromStr("15"),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					TradeLimitRatio:        sdk.OneDec(),
				},
			},
			isOverLimit: false,
		},
		{
			name: "lower limit of fluctuation limit",
			pool: Market{
				Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				QuoteReserve:  sdk.NewDec(999),
				BaseReserve:   sdk.OneDec(),
				PegMultiplier: sdk.OneDec(),
				Config: MarketConfig{
					FluctuationLimitRatio:  sdk.ZeroDec(),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.MustNewDecFromStr("15"),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					TradeLimitRatio:        sdk.OneDec(),
				},
			},
			isOverLimit: false,
		},
		{
			name: "upper limit of fluctuation limit",
			pool: Market{
				Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				QuoteReserve:  sdk.NewDec(1001),
				BaseReserve:   sdk.OneDec(),
				PegMultiplier: sdk.OneDec(),
				Config: MarketConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.001"),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.MustNewDecFromStr("15"),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					TradeLimitRatio:        sdk.OneDec(),
				},
			},
			isOverLimit: false,
		},
		{
			name: "under fluctuation limit",
			pool: Market{
				Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				QuoteReserve:  sdk.NewDec(998),
				BaseReserve:   sdk.OneDec(),
				PegMultiplier: sdk.OneDec(),
				Config: MarketConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.001"),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.MustNewDecFromStr("15"),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					TradeLimitRatio:        sdk.OneDec(),
				},
			},
			isOverLimit: true,
		},
		{
			name: "over fluctuation limit",
			pool: Market{
				Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				QuoteReserve:  sdk.NewDec(1002),
				BaseReserve:   sdk.OneDec(),
				PegMultiplier: sdk.OneDec(),
				Config: MarketConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.001"),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.MustNewDecFromStr("15"),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					TradeLimitRatio:        sdk.OneDec(),
				},
			},
			isOverLimit: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			snapshot := NewReserveSnapshot(
				asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				sdk.OneDec(),
				sdk.NewDec(1000),
				sdk.OneDec(),
				time.Now(),
			)
			assert.EqualValues(t, tc.isOverLimit, tc.pool.IsOverFluctuationLimitInRelationWithSnapshot(snapshot))
		})
	}
}

func TestMarket_ToSnapshot(t *testing.T) {
	tests := []struct {
		name       string
		market     Market
		expectFail bool
	}{
		{
			name: "happy path",
			market: Market{
				Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				BaseReserve:   sdk.NewDec(10),
				QuoteReserve:  sdk.NewDec(10_000),
				PegMultiplier: sdk.OneDec(),
			},
			expectFail: false,
		},
		{
			name: "err invalid base",
			market: Market{
				Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				BaseReserve:   sdk.Dec{},
				QuoteReserve:  sdk.NewDec(500),
				PegMultiplier: sdk.OneDec(),
			},
			expectFail: true,
		},
		{
			name: "err invalid quote",
			market: Market{
				Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				BaseReserve:   sdk.NewDec(500),
				QuoteReserve:  sdk.Dec{},
				PegMultiplier: sdk.OneDec(),
			},
			expectFail: true,
		},
		{
			name: "err negative quote",
			market: Market{
				Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				BaseReserve:   sdk.NewDec(500),
				QuoteReserve:  sdk.NewDec(-500),
				PegMultiplier: sdk.OneDec(),
			},
			expectFail: true,
		},
		{
			name: "err negative base",
			market: Market{
				Pair:          asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				BaseReserve:   sdk.NewDec(-500),
				QuoteReserve:  sdk.NewDec(500),
				PegMultiplier: sdk.OneDec(),
			},
			expectFail: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.BlankContext(StoreKey)
			if tc.expectFail {
				require.Panics(t, func() {
					_ = tc.market.ToSnapshot(ctx)
				})
			} else {
				snapshot := tc.market.ToSnapshot(ctx)
				assert.EqualValues(t, tc.market.Pair, snapshot.Pair)
				assert.EqualValues(t, tc.market.BaseReserve, snapshot.BaseReserve)
				assert.EqualValues(t, tc.market.QuoteReserve, snapshot.QuoteReserve)
				assert.EqualValues(t, ctx.BlockTime().UnixMilli(), snapshot.TimestampMs)
			}
		})
	}
}

func TestDefaultMarketConfig(t *testing.T) {
	marketCfg := DefaultMarketConfig()
	err := marketCfg.Validate()
	require.NoError(t, err)
}

func TestMarketConfigWith(t *testing.T) {
	marketCfg := DefaultMarketConfig()

	marketCfgUpdates := MarketConfig{
		TradeLimitRatio:        sdk.NewDec(12),
		FluctuationLimitRatio:  sdk.NewDec(34),
		MaxOracleSpreadRatio:   sdk.NewDec(56),
		MaintenanceMarginRatio: sdk.NewDec(78),
		MaxLeverage:            sdk.NewDec(910),
	}

	var newMarketCfg MarketConfig

	testCases := testutil.FunctionTestCases{
		{Name: "WithTradeLimitRatio", Test: func() {
			assert.NotEqualValues(t, marketCfgUpdates.TradeLimitRatio, marketCfg.TradeLimitRatio)
			newMarketCfg = *marketCfg.WithTradeLimitRatio(marketCfgUpdates.TradeLimitRatio)
			assert.EqualValues(t, marketCfgUpdates.TradeLimitRatio, newMarketCfg.TradeLimitRatio)
		}},
		{Name: "WithFluctuationLimitRatio", Test: func() {
			assert.NotEqualValues(t, marketCfgUpdates.FluctuationLimitRatio, marketCfg.FluctuationLimitRatio)
			newMarketCfg = *marketCfg.WithFluctuationLimitRatio(marketCfgUpdates.FluctuationLimitRatio)
			assert.EqualValues(t, marketCfgUpdates.FluctuationLimitRatio, newMarketCfg.FluctuationLimitRatio)
		}},
		{Name: "WithMaxOracleSpreadRatio", Test: func() {
			assert.NotEqualValues(t, marketCfgUpdates.MaxOracleSpreadRatio, marketCfg.MaxOracleSpreadRatio)
			newMarketCfg = *marketCfg.WithMaxOracleSpreadRatio(marketCfgUpdates.MaxOracleSpreadRatio)
			assert.EqualValues(t, marketCfgUpdates.MaxOracleSpreadRatio, newMarketCfg.MaxOracleSpreadRatio)
		}},
		{Name: "WithMaintenanceMarginRatio", Test: func() {
			assert.NotEqualValues(t, marketCfgUpdates.MaintenanceMarginRatio, marketCfg.MaintenanceMarginRatio)
			newMarketCfg = *marketCfg.WithMaintenanceMarginRatio(marketCfgUpdates.MaintenanceMarginRatio)
			assert.EqualValues(t, marketCfgUpdates.MaintenanceMarginRatio, newMarketCfg.MaintenanceMarginRatio)
		}},
		{Name: "WithMaxLeverage", Test: func() {
			assert.NotEqualValues(t, marketCfgUpdates.MaxLeverage, marketCfg.MaxLeverage)
			newMarketCfg = *marketCfg.WithMaxLeverage(marketCfgUpdates.MaxLeverage)
			assert.EqualValues(t, marketCfgUpdates.MaxLeverage, newMarketCfg.MaxLeverage)
		}},
	}

	testutil.RunFunctionTests(t, testCases)
}

func TestGetRepegCost(t *testing.T) {
	pair := asset.Registry.Pair(denoms.BTC, denoms.NUSD)
	tests := []struct {
		name string

		initialMarket Market
		newPeg        sdk.Dec

		expectedCost sdk.Dec
		shouldErr    bool
	}{
		{name: "zero bias -> zero cost",
			initialMarket: Market{
				Pair:          pair,
				BaseReserve:   sdk.NewDec(100),
				QuoteReserve:  sdk.NewDec(100),
				PegMultiplier: sdk.OneDec(),
				TotalLong:     sdk.ZeroDec(),
				TotalShort:    sdk.ZeroDec(),
			},
			newPeg:       sdk.NewDec(3),
			expectedCost: sdk.ZeroDec(),
			shouldErr:    false,
		},
		{name: "same peg -> zero cost",
			initialMarket: Market{
				Pair:          pair,
				BaseReserve:   sdk.NewDec(100),
				QuoteReserve:  sdk.NewDec(100),
				PegMultiplier: sdk.OneDec(),
				TotalLong:     sdk.ZeroDec(),
				TotalShort:    sdk.ZeroDec(),
			},
			newPeg:       sdk.OneDec(),
			expectedCost: sdk.ZeroDec(),
			shouldErr:    false,
		},
		{name: "new peg -> simple math",
			initialMarket: Market{
				Pair:          pair,
				BaseReserve:   sdk.NewDec(100),
				QuoteReserve:  sdk.NewDec(100),
				PegMultiplier: sdk.OneDec(),
				TotalLong:     sdk.NewDec(25),
				TotalShort:    sdk.ZeroDec(),
			},
			newPeg:       sdk.NewDec(2),
			expectedCost: sdk.NewDec(20), // 20 * (2 - 1)
			shouldErr:    false,
		},
		{name: "new peg -> simple math but negative bias",
			initialMarket: Market{
				Pair:          pair,
				BaseReserve:   sdk.NewDec(100),
				QuoteReserve:  sdk.NewDec(100),
				PegMultiplier: sdk.OneDec(),
				TotalLong:     sdk.ZeroDec(),
				TotalShort:    sdk.NewDec(20),
			},
			newPeg:       sdk.NewDec(2),
			expectedCost: sdk.NewDec(-25), // -20 * (2 - 1)
			shouldErr:    false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			market := tc.initialMarket

			cost, err := market.GetRepegCost(tc.newPeg)
			if tc.shouldErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.EqualValues(t, tc.expectedCost, cost)
			}
		})
	}
}
