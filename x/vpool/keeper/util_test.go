package keeper

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/store"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmdb "github.com/tendermint/tm-db"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/testutil/mock"

	"github.com/NibiruChain/nibiru/x/vpool/types"
)

func VpoolKeeper(t *testing.T, pricefeedKeeper types.PricefeedKeeper) (
	vpoolKeeper Keeper, ctx sdk.Context,
) {
	storeKey := sdk.NewKVStoreKey(types.StoreKey)

	db := tmdb.NewMemDB()
	stateStore := store.NewCommitMultiStore(db)
	stateStore.MountStoreWithDB(storeKey, sdk.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	vpoolKeeper = NewKeeper(
		codec.NewProtoCodec(codectypes.NewInterfaceRegistry()),
		storeKey, pricefeedKeeper,
	)
	ctx = sdk.NewContext(stateStore, tmproto.Header{}, false, log.NewNopLogger())

	return vpoolKeeper, ctx
}

// holds mocks for interfaces defined in vpool/types/expected_keepers.go
type mockedDependencies struct {
	mockPricefeedKeeper *mock.MockPricefeedKeeper
	mockAccountKeeper   *mock.MockAccountKeeper
}

func getKeeper(t *testing.T) (Keeper, mockedDependencies, sdk.Context) {
	db := tmdb.NewMemDB()
	commitMultiStore := store.NewCommitMultiStore(db)
	// Mount the KV store with the x/perp store key
	storeKey := sdk.NewKVStoreKey(types.StoreKey)
	commitMultiStore.MountStoreWithDB(storeKey, sdk.StoreTypeIAVL, db)
	// Mount Transient store
	transientStoreKey := sdk.NewTransientStoreKey("transient" + types.StoreKey)
	commitMultiStore.MountStoreWithDB(transientStoreKey, sdk.StoreTypeTransient, nil)
	// Mount Memory store
	memStoreKey := storetypes.NewMemoryStoreKey("mem" + types.StoreKey)
	commitMultiStore.MountStoreWithDB(memStoreKey, sdk.StoreTypeMemory, nil)

	require.NoError(t, commitMultiStore.LoadLatestVersion())

	protoCodec := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())

	ctrl := gomock.NewController(t)
	mockedAccountKeeper := mock.NewMockAccountKeeper(ctrl)
	mockedPricefeedKeeper := mock.NewMockPricefeedKeeper(ctrl)

	mockedAccountKeeper.
		EXPECT().GetModuleAddress(types.ModuleName).
		Return(authtypes.NewModuleAddress(types.ModuleName)).AnyTimes()

	k := NewKeeper(
		protoCodec,
		storeKey,
		mockedPricefeedKeeper,
	)

	ctx := sdk.NewContext(commitMultiStore, tmproto.Header{}, false, log.NewNopLogger())

	return k, mockedDependencies{
		mockPricefeedKeeper: mockedPricefeedKeeper,
		mockAccountKeeper:   mockedAccountKeeper,
	}, ctx
}

func getSamplePool() *types.Pool {
	ratioLimit, _ := sdk.NewDecFromStr("0.9")
	fluctuationLimit, _ := sdk.NewDecFromStr("0.1")
	maxOracleSpreadRatio := sdk.MustNewDecFromStr("0.1")
	maintenanceMarginRatio := sdk.MustNewDecFromStr("0.0625")
	maxLeverage := sdk.MustNewDecFromStr("15")

	pool := types.NewPool(
		common.PairBTCStable,
		ratioLimit,
		sdk.NewDec(10_000_000),
		sdk.NewDec(5_000_000),
		fluctuationLimit,
		maxOracleSpreadRatio,
		maintenanceMarginRatio,
		maxLeverage,
	)

	return pool
}
