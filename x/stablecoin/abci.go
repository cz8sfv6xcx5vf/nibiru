package stablecoin

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/stablecoin/keeper"
)

// EndBlocker updates the current pricefeed
func EndBlocker(ctx sdk.Context, k keeper.Keeper) {
	if !k.GetParams(ctx).IsCollateralRatioValid {
		// Try to re-start the collateral ratio updates
		err := k.EvaluateCollRatio(ctx)

		params := k.GetParams(ctx)
		params.IsCollateralRatioValid = err == nil

		k.SetParams(ctx, params)
	}

	_, err := k.PricefeedKeeper.GetCurrentTWAP(ctx, common.DenomStable, common.DenomColl)
	if err != nil {
		params := k.GetParams(ctx)
		params.IsCollateralRatioValid = false

		k.SetParams(ctx, params)
	}
}
