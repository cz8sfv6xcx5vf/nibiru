package stablecoin_test

import (
	"testing"
	"time"

	"github.com/NibiruChain/nibiru/x/testutil"

	"github.com/NibiruChain/nibiru/simapp"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/epochs"
	"github.com/NibiruChain/nibiru/x/pricefeed"
	ptypes "github.com/NibiruChain/nibiru/x/pricefeed/types"
)

type test struct {
	Name              string
	InCollRatio       sdk.Dec
	ExpectedCollRatio sdk.Dec
	price             sdk.Dec
	fn                func()
}

func TestEpochInfoChangesBeginBlockerAndInitGenesis(t *testing.T) {
	var app *simapp.NibiruTestApp
	var ctx sdk.Context

	tests := []test{
		{
			Name:              "Collateral price higher than stable, wait for correct amount of time",
			InCollRatio:       sdk.MustNewDecFromStr("0.8"),
			price:             sdk.MustNewDecFromStr("0.9"),
			ExpectedCollRatio: sdk.MustNewDecFromStr("0.7975"),
			fn: func() {
				ctx = ctx.WithBlockHeight(2).WithBlockTime(ctx.BlockTime().Add(time.Second))
				epochs.BeginBlocker(ctx, app.EpochsKeeper)

				ctx = ctx.WithBlockHeight(3).WithBlockTime(ctx.BlockTime().Add(time.Second * 60 * 16))
				epochs.BeginBlocker(ctx, app.EpochsKeeper)
			},
		},
		{
			Name:              "Price at peg, coll ratio should be the same",
			InCollRatio:       sdk.MustNewDecFromStr("0.8"),
			price:             sdk.MustNewDecFromStr("1"),
			ExpectedCollRatio: sdk.MustNewDecFromStr("0.8"),
			fn: func() {
				ctx = ctx.WithBlockHeight(2).WithBlockTime(ctx.BlockTime().Add(time.Second))
				epochs.BeginBlocker(ctx, app.EpochsKeeper)

				ctx = ctx.WithBlockHeight(3).WithBlockTime(ctx.BlockTime().Add(time.Second * 60 * 16))
				epochs.BeginBlocker(ctx, app.EpochsKeeper)
			},
		},
		{
			Name:              "Price higher than peg, but we don't wait for enough time, coll ratio should be the same",
			InCollRatio:       sdk.MustNewDecFromStr("0.8"),
			price:             sdk.MustNewDecFromStr("0.9"),
			ExpectedCollRatio: sdk.MustNewDecFromStr("0.8"),
			fn: func() {
				ctx = ctx.WithBlockHeight(2).WithBlockTime(ctx.BlockTime().Add(time.Second))
				epochs.BeginBlocker(ctx, app.EpochsKeeper)

				ctx = ctx.WithBlockHeight(3).WithBlockTime(ctx.BlockTime().Add(time.Second * 2))
				epochs.BeginBlocker(ctx, app.EpochsKeeper)

				ctx = ctx.WithBlockHeight(3).WithBlockTime(ctx.BlockTime().Add(time.Second * 3))
				epochs.BeginBlocker(ctx, app.EpochsKeeper)
			},
		},
		{
			Name:              "Collateral price higher than stable, and we wait for 2 updates, coll ratio should be updated twice",
			InCollRatio:       sdk.MustNewDecFromStr("0.8"),
			price:             sdk.MustNewDecFromStr("1.1"),
			ExpectedCollRatio: sdk.MustNewDecFromStr("0.805"),
			fn: func() {
				ctx = ctx.WithBlockHeight(2).WithBlockTime(ctx.BlockTime().Add(time.Second))
				epochs.BeginBlocker(ctx, app.EpochsKeeper)

				ctx = ctx.WithBlockHeight(3).WithBlockTime(ctx.BlockTime().Add(time.Second + time.Minute*15))
				epochs.BeginBlocker(ctx, app.EpochsKeeper)

				ctx = ctx.WithBlockHeight(3).WithBlockTime(ctx.BlockTime().Add(time.Second + time.Minute*30))
				epochs.BeginBlocker(ctx, app.EpochsKeeper)
			},
		},
		{
			Name:              "Collateral price higher than stable, and we wait for 2 updates but the last one is too close for update, coll ratio should be updated once",
			InCollRatio:       sdk.MustNewDecFromStr("0.8"),
			price:             sdk.MustNewDecFromStr("1.1"),
			ExpectedCollRatio: sdk.MustNewDecFromStr("0.8025"),
			fn: func() {
				ctx = ctx.WithBlockHeight(2).WithBlockTime(ctx.BlockTime().Add(time.Second))
				epochs.BeginBlocker(ctx, app.EpochsKeeper)

				ctx = ctx.WithBlockHeight(3).WithBlockTime(ctx.BlockTime().Add(time.Second + time.Minute*14))
				epochs.BeginBlocker(ctx, app.EpochsKeeper)

				ctx = ctx.WithBlockHeight(3).WithBlockTime(ctx.BlockTime().Add(time.Second + time.Minute*16))
				epochs.BeginBlocker(ctx, app.EpochsKeeper)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			app, ctx = simapp.NewTestNibiruAppAndContext(true)

			ctx = ctx.WithBlockHeight(1)

			oracle := testutil.AccAddress()
			pairs := common.AssetPairs{
				common.Pair_USDC_NUSD,
			}
			params := ptypes.NewParams(pairs, 2*time.Hour)
			app.PricefeedKeeper.SetParams(ctx, params)
			app.PricefeedKeeper.WhitelistOracles(ctx, []sdk.AccAddress{oracle})

			require.NoError(t, app.PricefeedKeeper.PostRawPrice(
				ctx,
				oracle,
				pairs[0].String(),
				/* price */ tc.price,
				/* expiry */ ctx.BlockTime().UTC().Add(time.Hour*1)))

			require.NoError(t, app.PricefeedKeeper.GatherRawPrices(ctx, pairs[0].Token0, pairs[0].Token1))
			require.NoError(t, app.StablecoinKeeper.SetCollRatio(ctx, tc.InCollRatio))

			tc.fn()

			currCollRatio := app.StablecoinKeeper.GetCollRatio(ctx)
			require.Equal(t, tc.ExpectedCollRatio, currCollRatio)
		})
	}
}

func TestEpochInfoChangesCollateralValidity(t *testing.T) {
	app, ctx := simapp.NewTestNibiruAppAndContext(true)

	runBlock := func(duration time.Duration) {
		ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1).WithBlockTime(ctx.BlockTime().Add(duration))
		pricefeed.BeginBlocker(ctx, app.PricefeedKeeper)
		epochs.BeginBlocker(ctx, app.EpochsKeeper)
	}

	// start at t=1sec with blockheight 1
	ctx = ctx.WithBlockHeight(1).WithBlockTime(time.Now())
	pricefeed.BeginBlocker(ctx, app.PricefeedKeeper)
	epochs.BeginBlocker(ctx, app.EpochsKeeper)

	oracle := testutil.AccAddress()
	pairs := common.AssetPairs{
		common.Pair_USDC_NUSD,
	}
	twapLookbackWindow := 1 * time.Hour
	params := ptypes.NewParams(pairs, twapLookbackWindow)
	app.PricefeedKeeper.SetParams(ctx, params)
	app.PricefeedKeeper.WhitelistOracles(ctx, []sdk.AccAddress{oracle})

	// Sim set price set the price for one hour
	require.NoError(t, app.PricefeedKeeper.PostRawPrice(
		ctx, oracle, pairs[0].String(), sdk.MustNewDecFromStr("0.9"), ctx.BlockTime().Add(time.Hour)))
	require.NoError(t, app.PricefeedKeeper.GatherRawPrices(ctx, pairs[0].Token0, pairs[0].Token1))
	require.NoError(t, app.StablecoinKeeper.SetCollRatio(ctx, sdk.MustNewDecFromStr("0.8")))

	// Mint block #2
	runBlock(time.Minute * 15)
	require.True(t, app.StablecoinKeeper.GetParams(ctx).IsCollateralRatioValid)

	// Mint block #3, collateral should be not valid because price are expired
	runBlock(time.Hour) // Collateral ratio is set to invalid at the beginning of this block
	require.False(t, app.StablecoinKeeper.GetParams(ctx).IsCollateralRatioValid)

	// Post price, collateral should be valid again
	require.NoError(t, app.PricefeedKeeper.PostRawPrice(
		ctx, oracle, pairs[0].String(), sdk.MustNewDecFromStr("0.9"), ctx.BlockTime().Add(time.Hour)))
	require.NoError(t, app.PricefeedKeeper.GatherRawPrices(ctx, pairs[0].Token0, pairs[0].Token1))

	// Mint block #4, median price and TWAP are computed again at the end of a new block
	runBlock(15 * time.Minute) // Collateral ratio is set to valid at the next epoch
	require.True(t, app.StablecoinKeeper.GetParams(ctx).IsCollateralRatioValid)
}
