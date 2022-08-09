package keeper

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/testutil/mock"
	"github.com/NibiruChain/nibiru/x/vpool/types"
)

func TestQueryReserveAssets(t *testing.T) {
	t.Log("initialize vpoolkeeper")
	vpoolKeeper, ctx := VpoolKeeper(t,
		mock.NewMockPricefeedKeeper(gomock.NewController(t)),
	)
	queryServer := NewQuerier(vpoolKeeper)

	t.Log("initialize vpool")
	pool := types.NewPool(
		/* pair */ common.PairBTCStable,
		/* tradeLimitRatio */ sdk.ZeroDec(),
		/* quoteAmount */ sdk.NewDec(1_000_000),
		/* baseAmount */ sdk.NewDec(1000),
		/* fluctuationLimitRatio */ sdk.ZeroDec(),
		/* maxOracleSpreadRatio */ sdk.ZeroDec(),
		/* maintenanceMarginRatio */ sdk.MustNewDecFromStr("0.0625"),
		/* maxLeverage */ sdk.MustNewDecFromStr("15"),
	)
	vpoolKeeper.savePool(ctx, pool)

	t.Log("query reserve assets")
	resp, err := queryServer.ReserveAssets(
		sdk.WrapSDKContext(ctx),
		&types.QueryReserveAssetsRequest{
			Pair: common.PairBTCStable.String(),
		},
	)

	t.Log("assert reserve assets")
	require.NoError(t, err)
	assert.EqualValues(t, pool.QuoteAssetReserve, resp.QuoteAssetReserve)
	assert.EqualValues(t, pool.BaseAssetReserve, resp.BaseAssetReserve)
}
