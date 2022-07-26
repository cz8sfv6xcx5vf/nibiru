package vpool_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/testutil/testapp"
	"github.com/NibiruChain/nibiru/x/vpool"
	"github.com/NibiruChain/nibiru/x/vpool/types"
)

// TODO: https://github.com/NibiruChain/nibiru/issues/475
func TestGenesis(t *testing.T) {
	vpools := []*types.Pool{
		{
			Pair:                   common.MustNewAssetPair("BTC:NUSD"),
			BaseAssetReserve:       sdk.NewDec(1_000_000),      // 1
			QuoteAssetReserve:      sdk.NewDec(30_000_000_000), // 30,000
			TradeLimitRatio:        sdk.MustNewDecFromStr("0.88"),
			FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.20"),
			MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.20"),
			MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
		},
		{
			Pair:                   common.MustNewAssetPair("ETH:NUSD"),
			BaseAssetReserve:       sdk.NewDec(2_000_000),      // 2
			QuoteAssetReserve:      sdk.NewDec(60_000_000_000), // 60,000
			TradeLimitRatio:        sdk.MustNewDecFromStr("0.77"),
			FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.30"),
			MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.30"),
			MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
		},
	}

	genesisState := types.GenesisState{Vpools: vpools}

	nibiruApp, ctx := testapp.NewNibiruAppAndContext(true)
	k := nibiruApp.VpoolKeeper
	vpool.InitGenesis(ctx, k, genesisState)

	for _, vp := range vpools {
		require.True(t, k.ExistsPool(ctx, vp.Pair))
	}

	exportedGenesis := vpool.ExportGenesis(ctx, k)
	require.Len(t, exportedGenesis.Vpools, 2)

	for _, exportedVpool := range exportedGenesis.Vpools {
		require.Contains(t, genesisState.Vpools, exportedVpool)
	}
}
