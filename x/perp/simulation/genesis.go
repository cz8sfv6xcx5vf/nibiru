package simulation

import (
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/perp/types"
)

// RandomizedGenState generates a random GenesisState for the perp module
func RandomizedGenState(simState *module.SimulationState) {
	perpGenesis := types.GenesisState{
		Params: types.DefaultParams(),
		PairMetadata: []types.PairMetadata{
			{
				Pair:                       common.Pair_BTC_NUSD,
				CumulativePremiumFractions: []sdk.Dec{sdk.ZeroDec()},
			},
		},
		Positions:       []types.Position{},
		PrepaidBadDebts: []types.PrepaidBadDebt{},
	}
	perpGenesisBytes, err := json.MarshalIndent(&perpGenesis, "", " ")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Generated perp genesis:\n%s\n", perpGenesisBytes)
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&perpGenesis)
}
