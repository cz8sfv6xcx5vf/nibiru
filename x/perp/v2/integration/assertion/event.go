package assertion

import (
	"errors"
	"fmt"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"

	"github.com/NibiruChain/nibiru/app"
	"github.com/NibiruChain/nibiru/x/common/testutil"
	"github.com/NibiruChain/nibiru/x/common/testutil/action"
	types "github.com/NibiruChain/nibiru/x/perp/v2/types"
)

var _ action.Action = (*containsLiquidateEvent)(nil)
var _ action.Action = (*positionChangedEventShouldBeEqual)(nil)

// TODO test(perp): Add action for testing the appearance of of successful
// liquidation events.

// --------------------------------------------------
// --------------------------------------------------

type containsLiquidateEvent struct {
	expectedEvent types.LiquidationFailedEvent
}

func (act containsLiquidateEvent) Do(_ *app.NibiruApp, ctx sdk.Context) (
	outCtx sdk.Context, err error, isMandatory bool,
) {
	isEventContained := false
	events := ctx.EventManager().Events()
	eventsOfMatchingType := []abci.Event{}
	for _, sdkEvent := range events {
		err := assertLiquidationFailedEvent(sdkEvent, act.expectedEvent)
		if err == nil {
			isEventContained = true
			break
		} else if sdkEvent.Type != "nibiru.perp.v2.LiquidationFailedEvent" {
			continue
		} else if sdkEvent.Type == "nibiru.perp.v2.LiquidationFailedEvent" && err != nil {
			abciEvent := abci.Event{
				Type:       sdkEvent.Type,
				Attributes: sdkEvent.Attributes,
			}
			eventsOfMatchingType = append(eventsOfMatchingType, abciEvent)
		}
	}

	if isEventContained {
		// happy path
		return ctx, nil, true
	} else {
		// Show descriptive error messages if the expected event is missing
		wantEventJson, _ := testutil.ProtoToJson(&act.expectedEvent)
		var matchingEvents string = sdk.StringifyEvents(eventsOfMatchingType).String()
		return ctx, errors.New(
			strings.Join([]string{
				fmt.Sprintf("expected the context event manager to contain event: %s.", wantEventJson),
				fmt.Sprintf("found %v events:", len(events)),
				fmt.Sprintf("events of matching type:\n%v", matchingEvents),
			}, "\n"),
		), false
	}
}

// ContainsLiquidateEvent checks if a typed event (proto.Message) is contained in the
// event manager of the app context.
func ContainsLiquidateEvent(
	expectedEvent types.LiquidationFailedEvent,
) action.Action {
	return containsLiquidateEvent{
		expectedEvent: expectedEvent,
	}
}

func assertLiquidationFailedEvent(
	sdkEvent sdk.Event, liquidationFailedEvent types.LiquidationFailedEvent,
) error {
	fieldErrs := []string{}

	for _, eventField := range []struct {
		key  string
		want string
	}{
		{"pair", liquidationFailedEvent.Pair.String()},
		{"trader", liquidationFailedEvent.Trader},
		{"liquidator", liquidationFailedEvent.Liquidator},
		{"reason", liquidationFailedEvent.Reason.String()},
	} {
		if err := testutil.EventHasAttribueValue(sdkEvent, eventField.key, eventField.want); err != nil {
			fieldErrs = append(fieldErrs, err.Error())
		}
	}

	if len(fieldErrs) > 0 {
		return errors.New(strings.Join(fieldErrs, ". "))
	}

	return nil
}

func assertPositionChangedEvent(
	sdkEvent sdk.Event, positionChangedEvent types.PositionChangedEvent,
) error {
	badDebtBz, err := codec.ProtoMarshalJSON(&positionChangedEvent.BadDebt, nil)
	if err != nil {
		panic(err)
	}
	transactionFeeBz, err := codec.ProtoMarshalJSON(&positionChangedEvent.TransactionFee, nil)
	if err != nil {
		panic(err)
	}

	fieldErrs := []string{}

	for _, eventField := range []struct {
		key  string
		want string
	}{
		{"position_notional", positionChangedEvent.PositionNotional.String()},
		{"transaction_fee", string(transactionFeeBz)},
		{"bad_debt", string(badDebtBz)},
		{"realized_pnl", positionChangedEvent.RealizedPnl.String()},
		{"funding_payment", positionChangedEvent.FundingPayment.String()},
		{"block_height", fmt.Sprintf("%v", positionChangedEvent.BlockHeight)},
		{"margin_to_user", positionChangedEvent.MarginToUser.String()},
		{"change_reason", string(positionChangedEvent.ChangeReason)},
	} {
		if err := testutil.EventHasAttribueValue(sdkEvent, eventField.key, eventField.want); err != nil {
			fieldErrs = append(fieldErrs, err.Error())
		}
	}

	if len(fieldErrs) > 0 {
		return errors.New(strings.Join(fieldErrs, ". "))
	}

	return nil
}

type positionChangedEventShouldBeEqual struct {
	ExpectedEvent *types.PositionChangedEvent
}

func (p positionChangedEventShouldBeEqual) Do(_ *app.NibiruApp, ctx sdk.Context) (sdk.Context, error, bool) {
	for _, gotSdkEvent := range ctx.EventManager().Events() {
		if gotSdkEvent.Type != proto.MessageName(p.ExpectedEvent) {
			continue
		}
		gotProtoMessage, err := sdk.ParseTypedEvent(abci.Event{
			Type:       gotSdkEvent.Type,
			Attributes: gotSdkEvent.Attributes,
		})
		if err != nil {
			return ctx, err, false
		}

		gotTypedEvent, ok := gotProtoMessage.(*types.PositionChangedEvent)
		if !ok {
			return ctx, fmt.Errorf("expected event is not of type PositionChangedEvent"), false
		}

		if err := types.PositionsAreEqual(&p.ExpectedEvent.FinalPosition, &gotTypedEvent.FinalPosition); err != nil {
			return ctx, err, false
		}

		if err := assertPositionChangedEvent(gotSdkEvent, *gotTypedEvent); err != nil {
			return ctx, err, false
		}
	}

	return ctx, nil, false
}

// PositionChangedEventShouldBeEqual checks that the position changed event is
// equal to the expected event.
func PositionChangedEventShouldBeEqual(
	expectedEvent *types.PositionChangedEvent,
) action.Action {
	return positionChangedEventShouldBeEqual{
		ExpectedEvent: expectedEvent,
	}
}
