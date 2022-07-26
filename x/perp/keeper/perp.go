package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/NibiruChain/nibiru/x/perp/types"
)

// SettlePosition settles a trader position
func (k Keeper) SettlePosition(
	ctx sdk.Context,
	currentPosition types.Position,
) (transferredCoins sdk.Coins, err error) {
	// Validate trader address
	traderAddr, err := sdk.AccAddressFromBech32(currentPosition.TraderAddress)
	if err != nil {
		return sdk.NewCoins(), nil
	}

	if currentPosition.Size_.IsZero() {
		return sdk.NewCoins(), nil
	}

	if err = k.PositionsState(ctx).Delete(
		currentPosition.Pair,
		traderAddr,
	); err != nil {
		return sdk.NewCoins(), nil
	}

	// run calculations on settled values
	settlementPrice, err := k.VpoolKeeper.GetSettlementPrice(ctx, currentPosition.Pair)
	if err != nil {
		return sdk.NewCoins(), nil
	}

	settledValue := sdk.ZeroDec()
	if settlementPrice.IsZero() {
		settledValue = currentPosition.Margin
	} else {
		// openPrice = positionOpenNotional / abs(positionSize)
		openPrice := currentPosition.OpenNotional.Quo(currentPosition.Size_.Abs())
		// returnedFund := positionSize * (settlementPrice - openPrice) + positionMargin
		returnedFund := currentPosition.Size_.Mul(
			settlementPrice.Sub(openPrice)).Add(currentPosition.Margin)
		if returnedFund.IsPositive() {
			settledValue = returnedFund
		}
	}

	transferredCoins = sdk.NewCoins(sdk.NewInt64Coin(currentPosition.Pair.QuoteDenom(), 0))
	settledValueInt := settledValue.RoundInt()
	if settledValueInt.IsPositive() {
		toTransfer := sdk.NewCoin(currentPosition.Pair.QuoteDenom(), settledValueInt)
		transferredCoins = sdk.NewCoins(toTransfer)
		if err != nil {
			panic(err) // NOTE(mercilex): must never happen
		}
		err = k.BankKeeper.SendCoinsFromModuleToAccount( // NOTE(mercilex): withdraw is not applied here
			ctx,
			types.VaultModuleAccount,
			traderAddr,
			transferredCoins,
		)
		if err != nil {
			panic(err) // NOTE(mercilex): must never happen
		}
	}

	err = ctx.EventManager().EmitTypedEvent(&types.PositionSettledEvent{
		Pair:          currentPosition.Pair.String(),
		TraderAddress: traderAddr.String(),
		SettledCoins:  transferredCoins,
	})

	return transferredCoins, err
}
