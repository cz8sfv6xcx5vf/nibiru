package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/spf13/cobra"

	"github.com/NibiruChain/nibiru/x/dex/types"

	"github.com/cosmos/cosmos-sdk/client/flags"
)

var _ = strconv.Itoa(0)

// GetQueryCmd returns the cli query commands for this module
func GetQueryCmd() *cobra.Command {
	// Group dex queries under a subcommand
	dexQueryCommand := &cobra.Command{
		Use: types.ModuleName,
		Short: fmt.Sprintf(
			"Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	commands := []*cobra.Command{
		CmdQueryParams(),
		CmdGetPoolNumber(),
		CmdGetPool(),
		CmdTotalLiquidity(),
		CmdTotalPoolLiquidity(),
	}

	for _, cmd := range commands {
		dexQueryCommand.AddCommand(cmd)
	}

	return dexQueryCommand
}

func CmdGetPoolNumber() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-pool-number",
		Short: "QueryPoolNumber",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			params := &types.QueryPoolNumberRequest{}

			res, err := queryClient.PoolNumber(cmd.Context(), params)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdGetPool() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-pool [pool-id]",
		Short: "Get a pool by its ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			poolId, _ := sdk.NewIntFromString(args[0])

			queryClient := types.NewQueryClient(clientCtx)

			params := &types.QueryPoolRequest{
				PoolId: poolId.Uint64(),
			}

			res, err := queryClient.Pool(cmd.Context(), params)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "shows the parameters of the module",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.Params(context.Background(), &types.QueryParamsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdTotalLiquidity() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "total-liquidity",
		Short: "Show liquidity of protocol",
		Long: strings.TrimSpace(
			fmt.Sprintf(`Query total-liquidity.
Example:
$ %s query dex total-liquidity
`, version.AppName,
			),
		),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.TotalLiquidity(context.Background(), &types.QueryTotalLiquidityRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdTotalPoolLiquidity() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pool-liquidity [pool-id]",
		Short: "Show liquidity of pool",
		Long: strings.TrimSpace(
			fmt.Sprintf(`Query total-liquidity.
Example:
$ %s query dex pool-liquidity 1
`, version.AppName,
			),
		),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)
			poolId, _ := sdk.NewIntFromString(args[0])

			res, err := queryClient.TotalPoolLiquidity(
				context.Background(),
				&types.QueryTotalPoolLiquidityRequest{PoolId: poolId.Uint64()},
			)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
