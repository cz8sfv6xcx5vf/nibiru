package keeper

import (
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/NibiruChain/nibiru/x/oracle/types"
)

// Tally calculates the median and returns it. Sets the set of voters to be rewarded, i.e. voted within
// a reasonable spread from the weighted median to the store
func Tally(ballot types.ExchangeRateBallot, rewardBand sdk.Dec, validatorPerformanceMap map[string]types.ValidatorPerformance) sdk.Dec {
	sort.Sort(ballot)

	weightedMedian := ballot.WeightedMedianWithAssertion()
	standardDeviation := ballot.StandardDeviation(weightedMedian)
	rewardSpread := weightedMedian.Mul(rewardBand.QuoInt64(2))

	if standardDeviation.GT(rewardSpread) {
		rewardSpread = standardDeviation
	}

	for _, vote := range ballot {
		// Filter ballot winners & abstain voters
		if (vote.ExchangeRate.GTE(weightedMedian.Sub(rewardSpread)) &&
			vote.ExchangeRate.LTE(weightedMedian.Add(rewardSpread))) ||
			!vote.ExchangeRate.IsPositive() {
			voterAddr := vote.Voter.String()

			validatorPerformance := validatorPerformanceMap[voterAddr]
			validatorPerformance.Weight += vote.Power
			validatorPerformance.WinCount++
			validatorPerformanceMap[voterAddr] = validatorPerformance
		}
	}

	return weightedMedian
}

// ballotIsPassingThreshold ballot for the asset is passing the threshold amount of voting power
func ballotIsPassingThreshold(ballot types.ExchangeRateBallot, thresholdVotes sdk.Int) bool {
	ballotPower := sdk.NewInt(ballot.Power())
	return !ballotPower.IsZero() && ballotPower.GTE(thresholdVotes)
}

// RemoveInvalidBallots removes the ballots which have not reached the vote threshold
// or which are not part of the whitelisted pairs anymore: example when params change during a vote period
// but some votes were already made.
func (k Keeper) RemoveInvalidBallots(
	ctx sdk.Context,
	pairBallotMap map[string]types.ExchangeRateBallot,
) (map[string]types.ExchangeRateBallot, map[string]struct{}) {
	whitelistedPairsMap := k.getWhitelistedPairsMap(ctx)

	totalBondedPower := sdk.TokensToConsensusPower(k.StakingKeeper.TotalBondedTokens(ctx), k.StakingKeeper.PowerReduction(ctx))
	voteThreshold := k.VoteThreshold(ctx)
	thresholdVotes := voteThreshold.MulInt64(totalBondedPower).RoundInt()

	for pair, ballot := range pairBallotMap {
		// If pair is not whitelisted, or the ballot for it has failed, then skip
		// and remove it from pairBallotMap for iteration efficiency
		if _, exists := whitelistedPairsMap[pair]; !exists {
			delete(pairBallotMap, pair)
			continue
		}

		// If the ballot is not passed, remove it from the voteTargets array
		// to prevent slashing validators who did valid vote.
		if !ballotIsPassingThreshold(ballot, thresholdVotes) {
			delete(whitelistedPairsMap, pair)
			delete(pairBallotMap, pair)
			continue
		}
	}

	return pairBallotMap, whitelistedPairsMap
}
