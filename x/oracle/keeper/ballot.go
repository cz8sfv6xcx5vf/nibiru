package keeper

import (
	"github.com/NibiruChain/nibiru/collections"

	"github.com/NibiruChain/nibiru/x/oracle/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// mapBallotByPair collects all oracle votes for the period, categorized by the votes' pair parameter
// and removes any votes that are not part of the validator performance map.
//
// NOTE: **Make abstain votes to have zero vote power**
func (k Keeper) mapBallotByPair(
	ctx sdk.Context,
	validatorsPerformance map[string]types.ValidatorPerformance,
) (ballots map[string]types.ExchangeRateBallot) {
	ballots = map[string]types.ExchangeRateBallot{}

	for _, value := range k.Votes.Iterate(ctx, collections.Range[sdk.ValAddress]{}).KeyValues() {
		voterAddr, vote := value.Key, value.Value

		// organize ballot only for the active validators
		if validatorPerformance, ok := validatorsPerformance[vote.Voter]; ok {
			for _, tuple := range vote.ExchangeRateTuples {
				power := validatorPerformance.Power
				if !tuple.ExchangeRate.IsPositive() {
					// Make the power of abstain vote zero
					power = 0
				}

				ballots[tuple.Pair] = append(ballots[tuple.Pair],
					types.NewBallotVoteForTally(
						tuple.ExchangeRate,
						tuple.Pair,
						voterAddr,
						power,
					),
				)
			}
		}
	}

	return
}

// clearBallots clears all tallied prevotes and votes from the store
func (k Keeper) clearBallots(ctx sdk.Context, votePeriod uint64) {
	// Clear all aggregate prevotes
	for _, prevote := range k.Prevotes.Iterate(ctx, collections.Range[sdk.ValAddress]{}).KeyValues() {
		if ctx.BlockHeight() > int64(prevote.Value.SubmitBlock+votePeriod) {
			err := k.Prevotes.Delete(ctx, prevote.Key)
			if err != nil {
				panic(err)
			}
		}
	}

	// Clear all aggregate votes
	for _, voteKey := range k.Votes.Iterate(ctx, collections.Range[sdk.ValAddress]{}).Keys() {
		err := k.Votes.Delete(ctx, voteKey)
		if err != nil {
			panic(err)
		}
	}
}

// applyWhitelist updates the whitelist by detecting possible changes between
// the current vote targets and the current updated whitelist.
func (k Keeper) applyWhitelist(ctx sdk.Context, whitelist []string, whitelistedPairsMap map[string]struct{}) {
	updateRequired := false

	if len(whitelistedPairsMap) != len(whitelist) {
		updateRequired = true
	} else {
		for _, pair := range whitelist {
			_, exists := whitelistedPairsMap[pair]
			if !exists {
				updateRequired = true
				break
			}
		}
	}

	if updateRequired {
		for _, p := range k.Pairs.Iterate(ctx, collections.Range[string]{}).Keys() {
			k.Pairs.Delete(ctx, p)
		}
		for _, pair := range whitelist {
			k.Pairs.Insert(ctx, pair)
		}
	}
}
