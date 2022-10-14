package keeper

import (
	"time"

	"github.com/NibiruChain/nibiru/collections"

	"github.com/NibiruChain/nibiru/x/common"

	sdk "github.com/cosmos/cosmos-sdk/types"

	epochstypes "github.com/NibiruChain/nibiru/x/epochs/types"
	"github.com/NibiruChain/nibiru/x/perp/types"
)

func (k Keeper) BeforeEpochStart(ctx sdk.Context, epochIdentifier string, epochNumber uint64) {
}

func (k Keeper) AfterEpochEnd(ctx sdk.Context, epochIdentifier string, _ uint64) {
	params := k.GetParams(ctx)
	if epochIdentifier != params.FundingRateInterval || params.Stopped {
		return
	}

	for _, pairMetadata := range k.PairsMetadata.Iterate(ctx, collections.Range[common.AssetPair]{}).Values() {
		if !k.VpoolKeeper.ExistsPool(ctx, pairMetadata.Pair) {
			ctx.Logger().Error("no pool for pair found", "pairMetadata.Pair", pairMetadata.Pair)
			continue
		}

		indexTWAP, err := k.PricefeedKeeper.GetCurrentTWAP(ctx, pairMetadata.Pair.Token0, pairMetadata.Pair.Token1)
		if err != nil {
			ctx.Logger().Error("failed to fetch twap index price", "pairMetadata.Pair", pairMetadata.Pair, "error", err)
			continue
		}
		if indexTWAP.IsZero() {
			ctx.Logger().Error("index price is zero", "pairMetadata.Pair", pairMetadata.Pair)
			continue
		}

		markTwap, err := k.VpoolKeeper.GetMarkPriceTWAP(ctx, pairMetadata.Pair, params.TwapLookbackWindow)
		if err != nil {
			ctx.Logger().Error("failed to fetch twap mark price", "pairMetadata.Pair", pairMetadata.Pair, "error", err)
			continue
		}
		if markTwap.IsZero() {
			ctx.Logger().Error("mark price is zero", "pairMetadata.Pair", pairMetadata.Pair)
			continue
		}

		epochInfo := k.EpochKeeper.GetEpochInfo(ctx, epochIdentifier)
		intervalsPerDay := (24 * time.Hour) / epochInfo.Duration
		// See https://www.notion.so/nibiru/Funding-Payments-5032d0f8ed164096808354296d43e1fa for an explanation of these terms.
		premiumFraction := markTwap.Sub(indexTWAP).QuoInt64(int64(intervalsPerDay))

		// If there is a previous cumulative funding rate, add onto that one. Otherwise, the funding rate is the first cumulative funding rate.
		cumulativePremiumFraction := premiumFraction
		if len(pairMetadata.CumulativePremiumFractions) > 0 {
			cumulativePremiumFraction = pairMetadata.CumulativePremiumFractions[len(pairMetadata.CumulativePremiumFractions)-1].Add(premiumFraction)
		}

		pairMetadata.CumulativePremiumFractions = append(pairMetadata.CumulativePremiumFractions, cumulativePremiumFraction)
		k.PairsMetadata.Insert(ctx, pairMetadata.Pair, pairMetadata)

		if err = ctx.EventManager().EmitTypedEvent(&types.FundingRateChangedEvent{
			Pair:                      pairMetadata.Pair.String(),
			MarkPrice:                 markTwap,
			IndexPrice:                indexTWAP,
			LatestFundingRate:         premiumFraction.Quo(indexTWAP),
			LatestPremiumFraction:     premiumFraction,
			CumulativePremiumFraction: cumulativePremiumFraction,
			BlockHeight:               ctx.BlockHeight(),
			BlockTimeMs:               ctx.BlockTime().UnixMilli(),
		}); err != nil {
			ctx.Logger().Error("failed to emit FundingRateChangedEvent", "pairMetadata.Pair", pairMetadata.Pair, "error", err)
			continue
		}
	}
}

// ___________________________________________________________________________________________________

// Hooks wrapper struct for perps keeper.
type Hooks struct {
	k Keeper
}

var _ epochstypes.EpochHooks = Hooks{}

// Hooks Return the wrapper struct.
func (k Keeper) Hooks() Hooks {
	return Hooks{k}
}

// BeforeEpochStart epochs hooks.
func (h Hooks) BeforeEpochStart(ctx sdk.Context, epochIdentifier string, epochNumber uint64) {
	h.k.BeforeEpochStart(ctx, epochIdentifier, epochNumber)
}

func (h Hooks) AfterEpochEnd(ctx sdk.Context, epochIdentifier string, epochNumber uint64) {
	h.k.AfterEpochEnd(ctx, epochIdentifier, epochNumber)
}
