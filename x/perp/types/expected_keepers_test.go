package types_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/NibiruChain/nibiru/x/perp/types"
	"github.com/NibiruChain/nibiru/x/testutil/testapp"
)

/*
TestExpectedKeepers verifies that the expected keeper interfaces in x/perp

	(see interfaces.go) are implemented on the corresponding app keeper,
	'NibiruApp.KeeperName'
*/
func TestExpectedKeepers(t *testing.T) {
	nibiruApp, _ := testapp.NewNibiruAppAndContext(true)
	testCases := []struct {
		name           string
		expectedKeeper interface{}
		appKeeper      interface{}
	}{
		{
			name:           "PricefeedKeeper from x/pricefeed",
			expectedKeeper: (*types.PricefeedKeeper)(nil),
			appKeeper:      nibiruApp.PricefeedKeeper,
		},
		{
			name:           "BankKeeper from the cosmos-sdk",
			expectedKeeper: (*types.BankKeeper)(nil),
			appKeeper:      nibiruApp.BankKeeper,
		},
		{
			name:           "AccountKeeper from the cosmos-sdk",
			expectedKeeper: (*types.AccountKeeper)(nil),
			appKeeper:      nibiruApp.AccountKeeper,
		},
		{
			name:           "VpoolKeeper from x/vpool",
			expectedKeeper: (*types.VpoolKeeper)(nil),
			appKeeper:      nibiruApp.VpoolKeeper,
		},
	}

	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			_interface := reflect.TypeOf(tc.expectedKeeper).Elem()
			isImplementingExpectedMethods := reflect.
				TypeOf(tc.appKeeper).Implements(_interface)
			assert.True(t, isImplementingExpectedMethods)
		})
	}
}
