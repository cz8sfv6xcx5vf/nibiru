package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultGenesis returns the default Capability genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:               DefaultParams(),
		PairMetadata:         []*PairMetadata(nil),
		Positions:            []*Position(nil),
		PrepaidBadDebts:      []*PrepaidBadDebt(nil),
		WhitelistedAddresses: []string(nil),
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}

	for i, pos := range gs.Positions {
		if err := pos.Validate(); err != nil {
			return fmt.Errorf("malformed genesis position %s at index %d: %w", pos, i, err)
		}
	}

	for i, addr := range gs.WhitelistedAddresses {
		if _, err := sdk.AccAddressFromBech32(addr); err != nil {
			return fmt.Errorf("malformed whitelisted address %s at index %d: %w", addr, i, err)
		}
	}

	for i, m := range gs.PairMetadata {
		if err := m.Validate(); err != nil {
			return fmt.Errorf("malformed pair metadata %s at index %d: %w", m, i, err)
		}
	}

	for i, m := range gs.PrepaidBadDebts {
		if err := m.Validate(); err != nil {
			return fmt.Errorf("malformed prepaid bad debt %s at index %d: %w", m, i, err)
		}
	}

	return nil
}
