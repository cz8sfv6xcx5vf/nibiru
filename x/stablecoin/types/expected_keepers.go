package types // noalias

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	dextypes "github.com/NibiruChain/nibiru/x/dex/types"
	pftypes "github.com/NibiruChain/nibiru/x/pricefeed/types"
)

// AccountKeeper defines the expected account keeper used for simulations (noalias)
type AccountKeeper interface {
	GetModuleAddress(name string) sdk.AccAddress
	GetModuleAccount(ctx sdk.Context, moduleName string) authtypes.ModuleAccountI
	GetAccount(ctx sdk.Context, addr sdk.AccAddress) authtypes.AccountI
	SetAccount(sdk.Context, authtypes.AccountI)
}

// BankKeeper defines the expected interface needed to retrieve account balances.
type BankKeeper interface {
	SpendableCoins(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins
	MintCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error
	SendCoinsFromAccountToModule(
		ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string,
		amt sdk.Coins,
	) error
	SendCoinsFromModuleToAccount(
		ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress,
		amt sdk.Coins,
	) error
	BurnCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error
	GetSupply(ctx sdk.Context, denom string) sdk.Coin
	GetBalance(ctx sdk.Context, addr sdk.AccAddress, denom string) sdk.Coin
	GetAllBalances(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins
}

type PricefeedKeeper interface {
	GetCurrentTWAP(ctx sdk.Context, token0 string, token1 string) (sdk.Dec, error)
	GetCurrentPrice(ctx sdk.Context, token0 string, token1 string,
	) (pftypes.CurrentPrice, error)
	GetCurrentPrices(ctx sdk.Context) pftypes.CurrentPrices
	GetRawPrices(ctx sdk.Context, marketId string) pftypes.PostedPrices
	IsWhitelistedOracle(ctx sdk.Context, pairID string, address sdk.AccAddress,
	) bool
	GetOraclesForPair(ctx sdk.Context, pairID string) (oracles []sdk.AccAddress)
	GatherRawPrices(ctx sdk.Context, token0 string, token1 string) error
}

type DexKeeper interface {
	GetFromPair(ctx sdk.Context, denomA string, denomB string,
	) (poolId uint64, err error)
	FetchPool(ctx sdk.Context, poolId uint64) (pool dextypes.Pool, err error)
}
