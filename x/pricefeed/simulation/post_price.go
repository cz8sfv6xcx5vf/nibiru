package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"github.com/NibiruChain/nibiru/x/pricefeed/keeper"
	"github.com/NibiruChain/nibiru/x/pricefeed/types"
)

func SimulateMsgPostPrice(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		msg := &types.MsgPostPrice{
			Oracle: simAccount.Address.String(),
		}

		// TODO: Handling the PostPrice simulation

		return simtypes.NoOpMsg(types.ModuleName, msg.Type(), "PostPrice simulation not implemented"), nil, nil
	}
}
