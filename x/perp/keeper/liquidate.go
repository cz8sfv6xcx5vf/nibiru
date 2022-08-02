package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/perp/types"
	vpooltypes "github.com/NibiruChain/nibiru/x/vpool/types"
)

/* Liquidate allows to liquidate the trader position if the margin is below the
required margin maintenance ratio.

args:
  - liquidatorAddr: the liquidator who is executing the liquidation
  - pair: the asset pair
  - traderAddr: the trader who owns the position being liquidated

ret:
  - feeToLiquidator: the amount of coins given to the liquidator
  - feeToFund: the amount of coins given to the ecosystem fund
  - err: error
*/
func (k Keeper) Liquidate(
	ctx sdk.Context,
	liquidatorAddr sdk.AccAddress,
	pair common.AssetPair,
	traderAddr sdk.AccAddress,
) (feeToLiquidator sdk.Coin, feeToFund sdk.Coin, err error) {
	err = k.requireVpool(ctx, pair)
	if err != nil {
		return sdk.Coin{}, sdk.Coin{}, err
	}

	position, err := k.PositionsState(ctx).Get(pair, traderAddr)
	if err != nil {
		return sdk.Coin{}, sdk.Coin{}, err
	}

	marginRatio, err := k.GetMarginRatio(
		ctx,
		*position,
		types.MarginCalculationPriceOption_MAX_PNL,
	)
	if err != nil {
		return sdk.Coin{}, sdk.Coin{}, err
	}

	if k.VpoolKeeper.IsOverSpreadLimit(ctx, pair) {
		marginRatioBasedOnOracle, err := k.GetMarginRatio(
			ctx, *position, types.MarginCalculationPriceOption_INDEX)
		if err != nil {
			return sdk.Coin{}, sdk.Coin{}, err
		}

		marginRatio = sdk.MaxDec(marginRatio, marginRatioBasedOnOracle)
	}

	params := k.GetParams(ctx)

	maintenanceMarginRatio := k.VpoolKeeper.GetMaintenanceMarginRatio(ctx, pair)
	err = requireMoreMarginRatio(marginRatio, maintenanceMarginRatio, false)
	if err != nil {
		return sdk.Coin{}, sdk.Coin{}, types.ErrMarginHighEnough
	}

	marginRatioBasedOnSpot, err := k.GetMarginRatio(
		ctx, *position, types.MarginCalculationPriceOption_SPOT)
	if err != nil {
		return sdk.Coin{}, sdk.Coin{}, err
	}

	var liquidationResponse types.LiquidateResp
	if marginRatioBasedOnSpot.GTE(params.LiquidationFeeRatio) {
		liquidationResponse, err = k.ExecutePartialLiquidation(ctx, liquidatorAddr, position)
	} else {
		liquidationResponse, err = k.ExecuteFullLiquidation(ctx, liquidatorAddr, position)
	}
	if err != nil {
		return sdk.Coin{}, sdk.Coin{}, err
	}

	feeToLiquidator = sdk.NewCoin(
		pair.QuoteDenom(),
		liquidationResponse.FeeToLiquidator,
	)

	feeToFund = sdk.NewCoin(
		pair.QuoteDenom(),
		liquidationResponse.FeeToPerpEcosystemFund,
	)

	return feeToLiquidator, feeToFund, nil
}

/*
Fully liquidates a position. It is assumed that the margin ratio has already been
checked prior to calling this method.

args:
  - ctx: cosmos-sdk context
  - liquidator: the liquidator's address
  - position: the position to liquidate

ret:
  - liquidationResp: a response object containing the results of the liquidation
  - err: error
*/
func (k Keeper) ExecuteFullLiquidation(
	ctx sdk.Context, liquidator sdk.AccAddress, position *types.Position,
) (liquidationResp types.LiquidateResp, err error) {
	params := k.GetParams(ctx)

	traderAddr, err := sdk.AccAddressFromBech32(position.TraderAddress)
	if err != nil {
		return types.LiquidateResp{}, err
	}

	positionResp, err := k.closePositionEntirely(
		ctx,
		/* currentPosition */ *position,
		/* quoteAssetAmountLimit */ sdk.ZeroDec(),
		/* skipFluctuationLimitCheck */ true,
	)
	if err != nil {
		return types.LiquidateResp{}, err
	}

	remainMargin := positionResp.MarginToVault.Abs()

	feeToLiquidator := params.LiquidationFeeRatio.
		Mul(positionResp.ExchangedNotionalValue).
		QuoInt64(2)
	totalBadDebt := positionResp.BadDebt

	if feeToLiquidator.GT(remainMargin) {
		// if the remainMargin is not enough for liquidationFee, count it as bad debt
		totalBadDebt = totalBadDebt.Add(feeToLiquidator.Sub(remainMargin))
		remainMargin = sdk.ZeroDec()
	} else {
		// Otherwise, the remaining margin will be transferred to ecosystemFund
		remainMargin = remainMargin.Sub(feeToLiquidator)
	}

	// Realize bad debt
	if totalBadDebt.IsPositive() {
		if err = k.realizeBadDebt(
			ctx,
			position.Pair.QuoteDenom(),
			totalBadDebt.RoundInt(),
		); err != nil {
			return types.LiquidateResp{}, err
		}
	}

	feeToPerpEcosystemFund := sdk.ZeroDec()
	if remainMargin.IsPositive() {
		feeToPerpEcosystemFund = remainMargin
	}

	liquidationResp = types.LiquidateResp{
		BadDebt:                totalBadDebt.RoundInt(),
		FeeToLiquidator:        feeToLiquidator.RoundInt(),
		FeeToPerpEcosystemFund: feeToPerpEcosystemFund.RoundInt(),
		Liquidator:             liquidator.String(),
		PositionResp:           positionResp,
	}
	err = k.distributeLiquidateRewards(ctx, liquidationResp)
	if err != nil {
		return types.LiquidateResp{}, err
	}

	markPrice, err := k.VpoolKeeper.GetSpotPrice(ctx, position.Pair)
	if err != nil {
		return types.LiquidateResp{}, err
	}

	err = ctx.EventManager().EmitTypedEvent(&types.PositionLiquidatedEvent{
		Pair:                  position.Pair.String(),
		TraderAddress:         traderAddr.String(),
		ExchangedQuoteAmount:  positionResp.ExchangedNotionalValue,
		ExchangedPositionSize: positionResp.ExchangedPositionSize,
		LiquidatorAddress:     liquidator.String(),
		FeeToLiquidator:       sdk.NewCoin(position.Pair.QuoteDenom(), feeToLiquidator.RoundInt()),
		FeeToEcosystemFund:    sdk.NewCoin(position.Pair.QuoteDenom(), feeToPerpEcosystemFund.RoundInt()),
		BadDebt:               sdk.NewCoin(position.Pair.QuoteDenom(), totalBadDebt.RoundInt()),
		Margin:                sdk.NewCoin(position.Pair.QuoteDenom(), liquidationResp.PositionResp.Position.Margin.RoundInt()),
		PositionNotional:      liquidationResp.PositionResp.PositionNotional,
		PositionSize:          liquidationResp.PositionResp.Position.Size_,
		UnrealizedPnl:         liquidationResp.PositionResp.UnrealizedPnlAfter,
		MarkPrice:             markPrice,
		BlockHeight:           ctx.BlockHeight(),
		BlockTimeMs:           ctx.BlockTime().UnixMilli(),
	})

	return liquidationResp, err
}

func (k Keeper) distributeLiquidateRewards(
	ctx sdk.Context, liquidateResp types.LiquidateResp) (err error) {
	// --------------------------------------------------------------
	//  Preliminary validations
	// --------------------------------------------------------------

	// validate response
	err = liquidateResp.Validate()
	if err != nil {
		return err
	}

	liquidator, err := sdk.AccAddressFromBech32(liquidateResp.Liquidator)
	if err != nil {
		return err
	}

	// validate pair
	pair := liquidateResp.PositionResp.Position.Pair
	err = k.requireVpool(ctx, pair)
	if err != nil {
		return err
	}

	// --------------------------------------------------------------
	// Distribution of rewards
	// --------------------------------------------------------------

	// Transfer fee from vault to PerpEF
	feeToPerpEF := liquidateResp.FeeToPerpEcosystemFund
	if feeToPerpEF.IsPositive() {
		coinToPerpEF := sdk.NewCoin(
			pair.QuoteDenom(), feeToPerpEF)
		if err = k.BankKeeper.SendCoinsFromModuleToModule(
			ctx,
			/* from */ types.VaultModuleAccount,
			/* to */ types.PerpEFModuleAccount,
			sdk.NewCoins(coinToPerpEF),
		); err != nil {
			return err
		}
	}

	// Transfer fee from vault to liquidator
	feeToLiquidator := liquidateResp.FeeToLiquidator
	if feeToLiquidator.IsPositive() {
		err = k.Withdraw(ctx, pair.QuoteDenom(), liquidator, feeToLiquidator)
		if err != nil {
			return err
		}
	}

	return nil
}

// ExecutePartialLiquidation partially liquidates a position
func (k Keeper) ExecutePartialLiquidation(
	ctx sdk.Context, liquidator sdk.AccAddress, currentPosition *types.Position,
) (types.LiquidateResp, error) {
	params := k.GetParams(ctx)

	traderAddr, err := sdk.AccAddressFromBech32(currentPosition.TraderAddress)
	if err != nil {
		return types.LiquidateResp{}, err
	}

	var baseAssetDir vpooltypes.Direction
	if currentPosition.Size_.IsPositive() {
		baseAssetDir = vpooltypes.Direction_ADD_TO_POOL
	} else {
		baseAssetDir = vpooltypes.Direction_REMOVE_FROM_POOL
	}

	partiallyLiquidatedPositionNotional, err := k.VpoolKeeper.GetBaseAssetPrice(
		ctx,
		currentPosition.Pair,
		baseAssetDir,
		/* abs= */ currentPosition.Size_.Mul(params.PartialLiquidationRatio),
	)
	if err != nil {
		return types.LiquidateResp{}, err
	}

	positionResp, err := k.decreasePosition(
		/* ctx */ ctx,
		/* currentPosition */ *currentPosition,
		/* quoteAssetAmount */ partiallyLiquidatedPositionNotional,
		/* baseAmtLimit */ sdk.ZeroDec(),
		/* skipFluctuationLimitCheck */ true,
	)
	if err != nil {
		return types.LiquidateResp{}, err
	}

	// Remove the liquidation fee from the margin of the position
	liquidationFeeAmount := positionResp.ExchangedNotionalValue.
		Mul(params.LiquidationFeeRatio)
	positionResp.Position.Margin = positionResp.Position.Margin.
		Sub(liquidationFeeAmount)
	k.PositionsState(ctx).Set(positionResp.Position)

	// Compute splits for the liquidation fee
	feeToLiquidator := liquidationFeeAmount.QuoInt64(2)
	feeToPerpEcosystemFund := liquidationFeeAmount.Sub(feeToLiquidator)

	liquidationResponse := types.LiquidateResp{
		BadDebt:                sdk.ZeroInt(),
		FeeToLiquidator:        feeToLiquidator.RoundInt(),
		FeeToPerpEcosystemFund: feeToPerpEcosystemFund.RoundInt(),
		Liquidator:             liquidator.String(),
		PositionResp:           positionResp,
	}
	err = k.distributeLiquidateRewards(ctx, liquidationResponse)
	if err != nil {
		return types.LiquidateResp{}, err
	}

	markPrice, err := k.VpoolKeeper.GetSpotPrice(ctx, currentPosition.Pair)
	if err != nil {
		return types.LiquidateResp{}, err
	}

	err = ctx.EventManager().EmitTypedEvent(&types.PositionLiquidatedEvent{
		Pair:                  currentPosition.Pair.String(),
		TraderAddress:         traderAddr.String(),
		ExchangedQuoteAmount:  positionResp.ExchangedNotionalValue,
		ExchangedPositionSize: positionResp.ExchangedPositionSize,
		LiquidatorAddress:     liquidator.String(),
		FeeToLiquidator:       sdk.NewCoin(currentPosition.Pair.QuoteDenom(), feeToLiquidator.RoundInt()),
		FeeToEcosystemFund:    sdk.NewCoin(currentPosition.Pair.QuoteDenom(), feeToPerpEcosystemFund.RoundInt()),
		BadDebt:               sdk.NewCoin(currentPosition.Pair.QuoteDenom(), liquidationResponse.BadDebt),
		Margin:                sdk.NewCoin(currentPosition.Pair.QuoteDenom(), liquidationResponse.PositionResp.Position.Margin.RoundInt()),
		PositionNotional:      liquidationResponse.PositionResp.PositionNotional,
		PositionSize:          liquidationResponse.PositionResp.Position.Size_,
		UnrealizedPnl:         liquidationResponse.PositionResp.UnrealizedPnlAfter,
		MarkPrice:             markPrice,
		BlockHeight:           ctx.BlockHeight(),
		BlockTimeMs:           ctx.BlockTime().UnixMilli(),
	})

	return liquidationResponse, err
}
