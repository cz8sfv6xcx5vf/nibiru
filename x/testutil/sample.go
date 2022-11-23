package testutil

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/store"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmdb "github.com/tendermint/tm-db"
)

// AccAddress Returns a sample account address (sdk.AccAddress)
// Note that AccAddress().String() can be used to get a string representation.
func AccAddress() sdk.AccAddress {
	pk := ed25519.GenPrivKey().PubKey()
	addr := pk.Address()
	return sdk.AccAddress(addr)
}

// PrivKeyAddressPairs generates (deterministically) a total of n private keys and addresses.
func PrivKeyAddressPairs(n int) (keys []cryptotypes.PrivKey, addrs []sdk.AccAddress) {
	r := rand.New(rand.NewSource(12345)) // make the generation deterministic
	keys = make([]cryptotypes.PrivKey, n)
	addrs = make([]sdk.AccAddress, n)
	for i := 0; i < n; i++ {
		secret := make([]byte, 32)
		_, err := r.Read(secret)
		if err != nil {
			panic("Could not read randomness")
		}
		keys[i] = ed25519.GenPrivKeyFromSecret(secret)
		addrs[i] = sdk.AccAddress(keys[i].PubKey().Address())
	}
	return
}

func BlankContext(storeKeyName string) sdk.Context {
	storeKey := sdk.NewKVStoreKey(storeKeyName)
	db := tmdb.NewMemDB()
	stateStore := store.NewCommitMultiStore(db)
	stateStore.MountStoreWithDB(storeKey, sdk.StoreTypeIAVL, db)
	ctx := sdk.NewContext(stateStore, tmproto.Header{}, false, log.NewNopLogger())
	return ctx
}
