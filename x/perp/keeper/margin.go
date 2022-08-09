package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/perp/types"
	vpooltypes "github.com/NibiruChain/nibiru/x/vpool/types"
)

/*
	AddMargin deleverages an existing position by adding margin (collateral)

to it. Adding margin increases the margin ratio of the corresponding position.
*/
func (k Keeper) AddMargin(
	ctx sdk.Context, pair common.AssetPair, traderAddr sdk.AccAddress, margin sdk.Coin,
) (res *types.MsgAddMarginResponse, err error) {
	// validate vpool exists
	if err = k.requireVpool(ctx, pair); err != nil {
		return nil, err
	}

	// ------------- AddMargin -------------
	position, err := k.PositionsState(ctx).Get(pair, traderAddr)
	if err != nil {
		return nil, err
	}

	remainingMargin, err := k.CalcRemainMarginWithFundingPayment(ctx, *position, margin.Amount.ToDec())
	if err != nil {
		return nil, err
	}

	if !remainingMargin.BadDebt.IsZero() {
		return nil, fmt.Errorf("failed to add margin; position has bad debt; consider adding more margin")
	}

	if err = k.BankKeeper.SendCoinsFromAccountToModule(
		ctx,
		/* from */ traderAddr,
		/* to */ types.VaultModuleAccount,
		/* amount */ sdk.NewCoins(margin),
	); err != nil {
		return nil, err
	}

	position.Margin = remainingMargin.Margin
	position.LastUpdateCumulativePremiumFraction = remainingMargin.LatestCumulativePremiumFraction
	position.BlockNumber = ctx.BlockHeight()
	k.PositionsState(ctx).Set(position)

	positionNotional, unrealizedPnl, err := k.getPositionNotionalAndUnrealizedPnL(ctx, *position, types.PnLCalcOption_SPOT_PRICE)
	if err != nil {
		return nil, err
	}

	spotPrice, err := k.VpoolKeeper.GetSpotPrice(ctx, pair)
	if err != nil {
		return nil, err
	}

	if err = ctx.EventManager().EmitTypedEvent(
		&types.PositionChangedEvent{
			Pair:                  pair.String(),
			TraderAddress:         traderAddr.String(),
			Margin:                sdk.NewCoin(pair.QuoteDenom(), position.Margin.RoundInt()),
			PositionNotional:      positionNotional,
			ExchangedPositionSize: sdk.ZeroDec(),                                 // always zero when adding margin
			TransactionFee:        sdk.NewCoin(pair.QuoteDenom(), sdk.ZeroInt()), // always zero when adding margin
			PositionSize:          position.Size_,
			RealizedPnl:           sdk.ZeroDec(), // always zero when adding margin
			UnrealizedPnlAfter:    unrealizedPnl,
			BadDebt:               sdk.NewCoin(pair.QuoteDenom(), remainingMargin.BadDebt.RoundInt()), // always zero when adding margin
			FundingPayment:        remainingMargin.FundingPayment,
			SpotPrice:             spotPrice,
			BlockHeight:           ctx.BlockHeight(),
			BlockTimeMs:           ctx.BlockTime().UnixMilli(),
			LiquidationPenalty:    sdk.ZeroDec(),
		},
	); err != nil {
		return nil, err
	}

	return &types.MsgAddMarginResponse{
		FundingPayment: remainingMargin.FundingPayment,
		Position:       position,
	}, nil
}

/*
	RemoveMargin further leverages an existing position by directly removing

the margin (collateral) that backs it from the vault. This also decreases the
margin ratio of the position.

Fails if the position goes underwater.

args:
  - ctx: the cosmos-sdk context
  - pair: the asset pair
  - traderAddr: the trader's address
  - margin: the amount of margin to withdraw. Must be positive.

ret:
  - marginOut: the amount of margin removed
  - fundingPayment: the funding payment that was applied with this position interaction
  - err: error if any
*/
func (k Keeper) RemoveMargin(
	ctx sdk.Context, pair common.AssetPair, traderAddr sdk.AccAddress, margin sdk.Coin,
) (marginOut sdk.Coin, fundingPayment sdk.Dec, position *types.Position, err error) {
	// validate vpool exists
	if err = k.requireVpool(ctx, pair); err != nil {
		return sdk.Coin{}, sdk.Dec{}, nil, err
	}

	// ------------- RemoveMargin -------------
	position, err = k.PositionsState(ctx).Get(pair, traderAddr)
	if err != nil {
		return sdk.Coin{}, sdk.Dec{}, nil, err
	}

	marginDelta := margin.Amount.Neg()
	remainingMargin, err := k.CalcRemainMarginWithFundingPayment(ctx, *position, marginDelta.ToDec())
	if err != nil {
		return sdk.Coin{}, sdk.Dec{}, nil, err
	}
	if !remainingMargin.BadDebt.IsZero() {
		return sdk.Coin{}, sdk.Dec{}, nil, types.ErrFailedRemoveMarginCanCauseBadDebt
	}

	position.Margin = remainingMargin.Margin
	position.LastUpdateCumulativePremiumFraction = remainingMargin.LatestCumulativePremiumFraction

	freeCollateral, err := k.calcFreeCollateral(ctx, *position)
	if err != nil {
		return sdk.Coin{}, sdk.Dec{}, nil, err
	} else if !freeCollateral.IsPositive() {
		return sdk.Coin{}, sdk.Dec{}, nil, fmt.Errorf("not enough free collateral")
	}

	k.PositionsState(ctx).Set(position)

	positionNotional, unrealizedPnl, err := k.getPositionNotionalAndUnrealizedPnL(ctx, *position, types.PnLCalcOption_SPOT_PRICE)
	if err != nil {
		return sdk.Coin{}, sdk.Dec{}, nil, err
	}

	spotPrice, err := k.VpoolKeeper.GetSpotPrice(ctx, pair)
	if err != nil {
		return sdk.Coin{}, sdk.Dec{}, nil, err
	}

	if err = k.Withdraw(ctx, pair.QuoteDenom(), traderAddr, margin.Amount); err != nil {
		return sdk.Coin{}, sdk.Dec{}, nil, err
	}

	if err = ctx.EventManager().EmitTypedEvent(
		&types.PositionChangedEvent{
			Pair:                  pair.String(),
			TraderAddress:         traderAddr.String(),
			Margin:                sdk.NewCoin(pair.QuoteDenom(), position.Margin.RoundInt()),
			PositionNotional:      positionNotional,
			ExchangedPositionSize: sdk.ZeroDec(),                                 // always zero when removing margin
			TransactionFee:        sdk.NewCoin(pair.QuoteDenom(), sdk.ZeroInt()), // always zero when removing margin
			PositionSize:          position.Size_,
			RealizedPnl:           sdk.ZeroDec(), // always zero when removing margin
			UnrealizedPnlAfter:    unrealizedPnl,
			BadDebt:               sdk.NewCoin(pair.QuoteDenom(), remainingMargin.BadDebt.RoundInt()), // always zero when removing margin
			FundingPayment:        remainingMargin.FundingPayment,
			SpotPrice:             spotPrice,
			BlockHeight:           ctx.BlockHeight(),
			BlockTimeMs:           ctx.BlockTime().UnixMilli(),
			LiquidationPenalty:    sdk.ZeroDec(),
		},
	); err != nil {
		return sdk.Coin{}, sdk.Dec{}, nil, err
	}

	return margin, remainingMargin.FundingPayment, position, nil
}

// GetMarginRatio calculates the MarginRatio from a Position
func (k Keeper) GetMarginRatio(
	ctx sdk.Context, position types.Position, priceOption types.MarginCalculationPriceOption,
) (marginRatio sdk.Dec, err error) {
	if position.Size_.IsZero() {
		return sdk.Dec{}, types.ErrPositionZero
	}

	var (
		unrealizedPnL    sdk.Dec
		positionNotional sdk.Dec
	)

	switch priceOption {
	case types.MarginCalculationPriceOption_MAX_PNL:
		positionNotional, unrealizedPnL, err = k.getPreferencePositionNotionalAndUnrealizedPnL(
			ctx,
			position,
			types.PnLPreferenceOption_MAX,
		)
	case types.MarginCalculationPriceOption_INDEX:
		positionNotional, unrealizedPnL, err = k.getPositionNotionalAndUnrealizedPnL(
			ctx,
			position,
			types.PnLCalcOption_ORACLE,
		)
	case types.MarginCalculationPriceOption_SPOT:
		positionNotional, unrealizedPnL, err = k.getPositionNotionalAndUnrealizedPnL(
			ctx,
			position,
			types.PnLCalcOption_SPOT_PRICE,
		)
	}

	if err != nil {
		return sdk.Dec{}, err
	}
	if positionNotional.IsZero() {
		// NOTE causes division by zero in margin ratio calculation
		return sdk.Dec{},
			fmt.Errorf("margin ratio doesn't make sense with zero position notional")
	}

	remaining, err := k.CalcRemainMarginWithFundingPayment(
		ctx,
		/* oldPosition */ position,
		/* marginDelta */ unrealizedPnL,
	)
	if err != nil {
		return sdk.Dec{}, err
	}

	marginRatio = remaining.Margin.Sub(remaining.BadDebt).
		Quo(positionNotional)
	return marginRatio, nil
}

func (k Keeper) requireVpool(ctx sdk.Context, pair common.AssetPair) (err error) {
	if !k.VpoolKeeper.ExistsPool(ctx, pair) {
		return types.ErrPairNotFound.Wrap(pair.String())
	}
	return nil
}

/*
requireMoreMarginRatio checks if the marginRatio corresponding to the margin
backing a position is above or below the 'baseMarginRatio'.
If 'largerThanOrEqualTo' is true, 'marginRatio' must be >= 'baseMarginRatio'.

Args:

	marginRatio: Ratio of the value of the margin and corresponding position(s).
	  marginRatio is defined as (margin + unrealizedPnL) / notional
	baseMarginRatio: Specifies the threshold value that 'marginRatio' must meet.
	largerThanOrEqualTo: Specifies whether 'marginRatio' should be larger or
	  smaller than 'baseMarginRatio'.
*/
func requireMoreMarginRatio(marginRatio, baseMarginRatio sdk.Dec, largerThanOrEqualTo bool) error {
	if largerThanOrEqualTo {
		if !marginRatio.GTE(baseMarginRatio) {
			return fmt.Errorf("margin ratio did not meet criteria")
		}
	} else {
		if !marginRatio.LT(baseMarginRatio) {
			return fmt.Errorf("margin ratio did not meet criteria")
		}
	}
	return nil
}

/*
Calculates position notional value and unrealized PnL. Lets the caller pick
either spot price, TWAP, or ORACLE to use for calculation.

args:
  - ctx: cosmos-sdk context
  - position: the trader's position
  - pnlCalcOption: SPOT or TWAP or ORACLE

Returns:
  - positionNotional: the position's notional value as sdk.Dec (signed)
  - unrealizedPnl: the position's unrealized profits and losses (PnL) as sdk.Dec (signed)
    For LONG positions, this is positionNotional - openNotional
    For SHORT positions, this is openNotional - positionNotional
*/
func (k Keeper) getPositionNotionalAndUnrealizedPnL(
	ctx sdk.Context,
	currentPosition types.Position,
	pnlCalcOption types.PnLCalcOption,
) (positionNotional sdk.Dec, unrealizedPnL sdk.Dec, err error) {
	positionSizeAbs := currentPosition.Size_.Abs()
	if positionSizeAbs.IsZero() {
		return sdk.ZeroDec(), sdk.ZeroDec(), nil
	}

	var baseAssetDirection vpooltypes.Direction
	if currentPosition.Size_.IsPositive() {
		// LONG
		baseAssetDirection = vpooltypes.Direction_ADD_TO_POOL
	} else {
		// SHORT
		baseAssetDirection = vpooltypes.Direction_REMOVE_FROM_POOL
	}

	switch pnlCalcOption {
	case types.PnLCalcOption_TWAP:
		positionNotional, err = k.VpoolKeeper.GetBaseAssetTWAP(
			ctx,
			currentPosition.Pair,
			baseAssetDirection,
			positionSizeAbs,
			/*lookbackInterval=*/ k.GetParams(ctx).TwapLookbackWindow,
		)
		if err != nil {
			k.Logger(ctx).Error(err.Error(), "calc_option", pnlCalcOption.String())
			return sdk.ZeroDec(), sdk.ZeroDec(), err
		}
	case types.PnLCalcOption_SPOT_PRICE:
		positionNotional, err = k.VpoolKeeper.GetBaseAssetPrice(
			ctx,
			currentPosition.Pair,
			baseAssetDirection,
			positionSizeAbs,
		)
		if err != nil {
			k.Logger(ctx).Error(err.Error(), "calc_option", pnlCalcOption.String())
			return sdk.ZeroDec(), sdk.ZeroDec(), err
		}
	case types.PnLCalcOption_ORACLE:
		oraclePrice, err := k.VpoolKeeper.GetUnderlyingPrice(
			ctx, currentPosition.Pair)
		if err != nil {
			k.Logger(ctx).Error(err.Error(), "calc_option", pnlCalcOption.String())
			return sdk.ZeroDec(), sdk.ZeroDec(), err
		}
		positionNotional = oraclePrice.Mul(positionSizeAbs)
	default:
		panic("unrecognized pnl calc option: " + pnlCalcOption.String())
	}

	if positionNotional.Equal(currentPosition.OpenNotional) {
		// if position notional and open notional are the same, then early return
		return positionNotional, sdk.ZeroDec(), nil
	}

	if currentPosition.Size_.IsPositive() {
		// LONG
		unrealizedPnL = positionNotional.Sub(currentPosition.OpenNotional)
	} else {
		// SHORT
		unrealizedPnL = currentPosition.OpenNotional.Sub(positionNotional)
	}

	k.Logger(ctx).Debug("get_position_notional_and_unrealized_pnl",
		"position",
		currentPosition.String(),
		"position_notional",
		positionNotional.String(),
		"unrealized_pnl",
		unrealizedPnL.String(),
	)

	return positionNotional, unrealizedPnL, nil
}

/*
Calculates both position notional value and unrealized PnL based on
both spot price and TWAP, and lets the caller pick which one based on MAX or MIN.

args:
  - ctx: cosmos-sdk context
  - position: the trader's position
  - pnlPreferenceOption: MAX or MIN

Returns:
  - positionNotional: the position's notional value as sdk.Dec (signed)
  - unrealizedPnl: the position's unrealized profits and losses (PnL) as sdk.Dec (signed)
    For LONG positions, this is positionNotional - openNotional
    For SHORT positions, this is openNotional - positionNotional
*/
func (k Keeper) getPreferencePositionNotionalAndUnrealizedPnL(
	ctx sdk.Context,
	position types.Position,
	pnLPreferenceOption types.PnLPreferenceOption,
) (positionNotional sdk.Dec, unrealizedPnl sdk.Dec, err error) {
	spotPositionNotional, spotPricePnl, err := k.getPositionNotionalAndUnrealizedPnL(
		ctx,
		position,
		types.PnLCalcOption_SPOT_PRICE,
	)
	if err != nil {
		k.Logger(ctx).Error(
			err.Error(),
			"calc_option",
			types.PnLCalcOption_SPOT_PRICE.String(),
			"preference_option",
			pnLPreferenceOption.String(),
		)
		return sdk.Dec{}, sdk.Dec{}, err
	}

	twapPositionNotional, twapPnl, err := k.getPositionNotionalAndUnrealizedPnL(
		ctx,
		position,
		types.PnLCalcOption_TWAP,
	)
	if err != nil {
		k.Logger(ctx).Error(
			err.Error(),
			"calc_option",
			types.PnLCalcOption_TWAP.String(),
			"preference_option",
			pnLPreferenceOption.String(),
		)
		return sdk.Dec{}, sdk.Dec{}, err
	}

	switch pnLPreferenceOption {
	case types.PnLPreferenceOption_MAX:
		positionNotional = sdk.MaxDec(spotPositionNotional, twapPositionNotional)
		unrealizedPnl = sdk.MaxDec(spotPricePnl, twapPnl)
	case types.PnLPreferenceOption_MIN:
		positionNotional = sdk.MinDec(spotPositionNotional, twapPositionNotional)
		unrealizedPnl = sdk.MinDec(spotPricePnl, twapPnl)
	default:
		panic("invalid pnl preference option " + pnLPreferenceOption.String())
	}

	return positionNotional, unrealizedPnl, nil
}
