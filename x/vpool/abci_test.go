package vpool_test

import (
	"testing"
	"time"

	"github.com/NibiruChain/collections"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NibiruChain/nibiru/x/common/asset"
	"github.com/NibiruChain/nibiru/x/common/denoms"
	"github.com/NibiruChain/nibiru/x/common/testutil"
	"github.com/NibiruChain/nibiru/x/common/testutil/testapp"
	"github.com/NibiruChain/nibiru/x/vpool"
	"github.com/NibiruChain/nibiru/x/vpool/types"
)

func TestSnapshotUpdates(t *testing.T) {
	nibiruApp, ctx := testapp.NewNibiruTestAppAndContext(true)
	vpoolKeeper := nibiruApp.VpoolKeeper

	runBlock := func(duration time.Duration) {
		vpool.EndBlocker(ctx, nibiruApp.VpoolKeeper)
		ctx = ctx.
			WithBlockHeight(ctx.BlockHeight() + 1).
			WithBlockTime(ctx.BlockTime().Add(duration))
	}

	ctx = ctx.WithBlockTime(time.Date(2015, 10, 21, 0, 0, 0, 0, time.UTC)).WithBlockHeight(1)

	require.NoError(t, vpoolKeeper.CreatePool(
		ctx,
		asset.Registry.Pair(denoms.BTC, denoms.NUSD),
		sdk.NewDec(1_000),
		sdk.NewDec(1_000),
		*types.DefaultVpoolConfig().
			WithTradeLimitRatio(sdk.OneDec()).
			WithFluctuationLimitRatio(sdk.OneDec()),
		sdk.ZeroDec(),
	))
	expectedSnapshot := types.NewReserveSnapshot(
		asset.Registry.Pair(denoms.BTC, denoms.NUSD),
		sdk.NewDec(1_000),
		sdk.NewDec(1_000),
		ctx.BlockTime(),
	)

	t.Log("run one block of 5 seconds")
	runBlock(5 * time.Second)
	snapshot, err := vpoolKeeper.ReserveSnapshots.Get(ctx, collections.Join(asset.Registry.Pair(denoms.BTC, denoms.NUSD), time.UnixMilli(expectedSnapshot.TimestampMs)))
	require.NoError(t, err)
	assert.EqualValues(t, expectedSnapshot, snapshot)

	t.Log("affect mark price")
	vpool, err := vpoolKeeper.GetPool(ctx, asset.Registry.Pair(denoms.BTC, denoms.NUSD))
	require.NoError(t, err)
	_, baseAmtAbs, err := vpoolKeeper.SwapQuoteForBase(
		ctx,
		vpool,
		types.Direction_ADD_TO_POOL,
		sdk.NewDec(250), // ← dyAmm
		sdk.ZeroDec(),
		false,
	)
	// dxAmm := (k / (y + dyAmm)) - x = (1e6 / (1e3 + 250)) - 1e3 = -200
	assert.EqualValues(t, sdk.NewDec(200), baseAmtAbs)
	require.NoError(t, err)
	expectedSnapshot = types.NewReserveSnapshot(
		asset.Registry.Pair(denoms.BTC, denoms.NUSD),
		sdk.NewDec(800),   // ← x + dxAmm
		sdk.NewDec(1_250), // ← y + dyAMM
		ctx.BlockTime(),
	)

	t.Log("run one block of 5 seconds")
	ctxAtSnapshot := sdk.Context(ctx) // manually copy ctx before the time skip
	timeSkipDuration := 5 * time.Second
	runBlock(timeSkipDuration) // increments ctx.blockHeight and ctx.BlockTime
	snapshot, err = vpoolKeeper.ReserveSnapshots.Get(ctx,
		collections.Join(asset.Registry.Pair(denoms.BTC, denoms.NUSD), time.UnixMilli(expectedSnapshot.TimestampMs)))
	require.NoError(t, err)
	assert.EqualValues(t, expectedSnapshot, snapshot)

	testutil.RequireContainsTypedEvent(t, ctx, &types.ReserveSnapshotSavedEvent{
		Pair:           expectedSnapshot.Pair,
		QuoteReserve:   expectedSnapshot.QuoteAssetReserve,
		BaseReserve:    expectedSnapshot.BaseAssetReserve,
		MarkPrice:      snapshot.QuoteAssetReserve.Quo(snapshot.BaseAssetReserve),
		BlockHeight:    ctxAtSnapshot.BlockHeight(),
		BlockTimestamp: ctxAtSnapshot.BlockTime(),
	})
}
