package keeper

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/common/asset"
	"github.com/NibiruChain/nibiru/x/common/denoms"
	"github.com/NibiruChain/nibiru/x/oracle/types"
)

func TestKeeperRewardsDistributionMultiVotePeriods(t *testing.T) {
	// this simulates allocating rewards for the pair nibi:nusd
	// over 5 voting periods. It simulates rewards are correctly
	// distributed over 5 voting periods to 5 validators.
	// then we simulate that after the 5 voting periods are
	// finished no more rewards distribution happen.
	const periods uint64 = 5
	const validators = 5

	fixture, msgServer := Setup(t)
	votePeriod := fixture.OracleKeeper.VotePeriod(fixture.Ctx)

	rewards := sdk.NewInt64Coin("reward", 1*common.TO_MICRO)
	valPeriodicRewards := sdk.NewDecCoinsFromCoins(rewards).
		QuoDec(sdk.NewDec(int64(periods))).
		QuoDec(sdk.NewDec(int64(validators)))
	AllocateRewards(t, fixture, asset.Registry.Pair(denoms.NIBI, denoms.NUSD), sdk.NewCoins(rewards), periods)

	for i := uint64(1); i <= periods; i++ {
		for valIndex := 0; valIndex < validators; valIndex++ {
			// for doc's sake, this function is capable of making prevotes and votes because it
			// passes the current context block height for pre vote
			// then changes the height to current height + vote period for the vote
			MakeAggregatePrevoteAndVote(t, fixture, msgServer, fixture.Ctx.BlockHeight(), types.ExchangeRateTuples{
				{
					Pair:         asset.Registry.Pair(denoms.NIBI, denoms.NUSD),
					ExchangeRate: randomExchangeRate,
				},
			}, valIndex)
		}

		// input.Ctx = input.Ctx.WithBlockHeight(input.Ctx.BlockHeight() + 1)
		fixture.OracleKeeper.UpdateExchangeRates(fixture.Ctx)

		for valIndex := 0; valIndex < validators; valIndex++ {
			distributionRewards := fixture.DistrKeeper.GetValidatorOutstandingRewards(fixture.Ctx, ValAddrs[0])
			truncatedGot, _ := distributionRewards.Rewards.
				QuoDec(sdk.NewDec(int64(i))). // outstanding rewards will count for the previous vote period too, so we divide it by current period
				TruncateDecimal()             // NOTE: not applying this on truncatedExpected because of rounding the test fails
			truncatedExpected, _ := valPeriodicRewards.TruncateDecimal()

			require.Equalf(t, truncatedExpected, truncatedGot, "period: %d, %s <-> %s", i, truncatedExpected.String(), truncatedGot.String())
		}
		// assert rewards

		fixture.Ctx = fixture.Ctx.WithBlockHeight(fixture.Ctx.BlockHeight() + int64(votePeriod))
	}

	// assert there are no rewards for pair
	require.True(t, fixture.OracleKeeper.GatherRewardsForVotePeriod(fixture.Ctx, asset.Registry.Pair(denoms.NIBI, denoms.NUSD)).IsZero())

	// assert that there are no rewards instances
	require.Empty(t, fixture.OracleKeeper.PairRewards.Indexes.RewardsByPair.ExactMatch(fixture.Ctx, asset.Registry.Pair(denoms.NIBI, denoms.NUSD)).PrimaryKeys())
}

func TestAllocateRewardsForUnlistedPair(t *testing.T) {
	fixture, _ := Setup(t)

	assert.Error(t, fixture.OracleKeeper.AllocatePairRewards(
		fixture.Ctx,
		faucetAccountName,
		asset.Registry.Pair("foo", "bar"), // pair doesn't exist
		sdk.NewCoins(sdk.NewInt64Coin("reward", 1*common.TO_MICRO)),
		1,
	))
}
