package core

import (
	"testing"

	evbus "github.com/asaskevich/EventBus"
	"github.com/benbjohnson/clock"
	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/core/coordinator"
	"github.com/evcc-io/evcc/core/settings"
	"github.com/evcc-io/evcc/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type authorizeChargerT struct {
	*api.MockCharger
	*api.MockStatusReasoner
	*api.MockAuthorizer
}

type authorizeVehicleT struct {
	*api.MockVehicle
	*api.MockChargeState
}

func newAuthorizeCharger(ctrl *gomock.Controller, reason api.Reason) *authorizeChargerT {
	c := &authorizeChargerT{api.NewMockCharger(ctrl), api.NewMockStatusReasoner(ctrl), api.NewMockAuthorizer(ctrl)}
	c.MockStatusReasoner.EXPECT().StatusReason().Return(reason, nil).AnyTimes()
	return c
}

func newAuthorizeVehicle(ctrl *gomock.Controller, status api.ChargeStatus, ids ...string) *authorizeVehicleT {
	v := &authorizeVehicleT{api.NewMockVehicle(ctrl), api.NewMockChargeState(ctrl)}
	v.MockVehicle.EXPECT().GetTitle().Return("vehicle").AnyTimes()
	v.MockVehicle.EXPECT().Features().AnyTimes()
	v.MockVehicle.EXPECT().Identifiers().Return(ids).AnyTimes()
	v.MockChargeState.EXPECT().Status().Return(status, nil).AnyTimes()
	return v
}

func newAuthorizeLoadpoint(charger api.Charger, vehicles ...api.Vehicle) *Loadpoint {
	lp := &Loadpoint{
		log:     util.NewLogger("foo"),
		bus:     evbus.New(),
		clock:   clock.NewMock(),
		charger: charger,
		status:  api.StatusB,
	}
	lp.coordinator = coordinator.NewAdapter(lp, coordinator.New(util.NewLogger("foo"), vehicles))
	lp.connectedTime = lp.clock.Now()
	return lp
}

func TestAuthorizeByVehicleStatus(t *testing.T) {
	ctrl := gomock.NewController(t)

	charger := newAuthorizeCharger(ctrl, api.ReasonWaitingForAuthorization)
	vehicle := newAuthorizeVehicle(ctrl, api.StatusB, "rfid*", "vid")
	lp := newAuthorizeLoadpoint(charger, vehicle)

	charger.MockAuthorizer.EXPECT().Authorize("vid").Return(nil).Times(1)
	lp.authorizeByVehicleStatus()

	// throttled until the next detection interval
	lp.authorizeByVehicleStatus()
}

func TestAuthorizeByVehicleStatusNotConnected(t *testing.T) {
	ctrl := gomock.NewController(t)

	charger := newAuthorizeCharger(ctrl, api.ReasonWaitingForAuthorization)
	dflt := newAuthorizeVehicle(ctrl, api.StatusA, "vid")
	lp := newAuthorizeLoadpoint(charger, dflt)

	// default vehicle is active, but the api does not confirm the connection
	lp.vehicle = dflt
	lp.defaultVehicle = dflt

	charger.MockAuthorizer.EXPECT().Authorize(gomock.Any()).Times(0)
	lp.authorizeByVehicleStatus()
}

func TestAuthorizeByVehicleStatusAfterDetectionWindow(t *testing.T) {
	ctrl := gomock.NewController(t)

	charger := newAuthorizeCharger(ctrl, api.ReasonWaitingForAuthorization)
	vehicle := newAuthorizeVehicle(ctrl, api.StatusB, "vid")
	lp := newAuthorizeLoadpoint(charger, vehicle)
	lp.clock.(*clock.Mock).Add(vehicleDetectDuration + 1)

	charger.MockAuthorizer.EXPECT().Authorize(gomock.Any()).Times(0)
	lp.authorizeByVehicleStatus()
}

func TestAuthorizeByVehicleStatusNotWaiting(t *testing.T) {
	ctrl := gomock.NewController(t)

	charger := newAuthorizeCharger(ctrl, api.ReasonUnknown)
	vehicle := newAuthorizeVehicle(ctrl, api.StatusB, "vid")
	lp := newAuthorizeLoadpoint(charger, vehicle)

	charger.MockAuthorizer.EXPECT().Authorize(gomock.Any()).Times(0)
	lp.authorizeByVehicleStatus()
}

func TestAuthorizeManual(t *testing.T) {
	ctrl := gomock.NewController(t)

	charger := newAuthorizeCharger(ctrl, api.ReasonWaitingForAuthorization)
	lp := newAuthorizeLoadpoint(charger)

	charger.MockAuthorizer.EXPECT().Authorize("").Return(nil).Times(1)
	assert.NoError(t, lp.Authorize(""))
}

func TestAuthorizeManualPin(t *testing.T) {
	ctrl := gomock.NewController(t)

	charger := newAuthorizeCharger(ctrl, api.ReasonWaitingForAuthorization)
	lp := newAuthorizeLoadpoint(charger)
	lp.authorizePin = "1234"

	assert.ErrorIs(t, lp.Authorize(""), ErrAuthorizeInvalidPin)
	assert.ErrorIs(t, lp.Authorize("0000"), ErrAuthorizeInvalidPin)

	charger.MockAuthorizer.EXPECT().Authorize("").Return(nil).Times(1)
	assert.NoError(t, lp.Authorize("1234"))
}

func TestAuthorizeManualLockout(t *testing.T) {
	ctrl := gomock.NewController(t)

	charger := newAuthorizeCharger(ctrl, api.ReasonWaitingForAuthorization)
	lp := newAuthorizeLoadpoint(charger)
	lp.authorizePin = "1234"

	for range authorizeMaxFailures {
		require.ErrorIs(t, lp.Authorize("0000"), ErrAuthorizeInvalidPin)
	}

	// correct pin is rejected while locked
	assert.ErrorIs(t, lp.Authorize("1234"), ErrAuthorizeLocked)

	lp.clock.(*clock.Mock).Add(authorizeLockout)

	charger.MockAuthorizer.EXPECT().Authorize("").Return(nil).Times(1)
	assert.NoError(t, lp.Authorize("1234"))
}

func TestAuthorizeManualNotWaiting(t *testing.T) {
	ctrl := gomock.NewController(t)

	charger := newAuthorizeCharger(ctrl, api.ReasonUnknown)
	lp := newAuthorizeLoadpoint(charger)

	assert.ErrorIs(t, lp.Authorize(""), ErrAuthorizeNotWaiting)
}

func TestAuthorizeManualNotSupported(t *testing.T) {
	ctrl := gomock.NewController(t)

	lp := newAuthorizeLoadpoint(api.NewMockCharger(ctrl))

	assert.ErrorIs(t, lp.Authorize(""), ErrAuthorizeNotSupported)
}

func TestSetAuthorizePin(t *testing.T) {
	lp := NewLoadpoint(util.NewLogger("foo"), settings.NewDatabaseSettingsAdapter("foo"))

	for _, pin := range []string{"123", "12345", "abcd", " 123"} {
		assert.Error(t, lp.SetAuthorizePin(pin), pin)
	}

	assert.NoError(t, lp.SetAuthorizePin("0815"))
	assert.Equal(t, "0815", lp.GetAuthorizePin())

	assert.NoError(t, lp.SetAuthorizePin(""))
	assert.Empty(t, lp.GetAuthorizePin())
}

func TestAuthorizeReusesVehicleDetection(t *testing.T) {
	ctrl := gomock.NewController(t)

	charger := newAuthorizeCharger(ctrl, api.ReasonWaitingForAuthorization)

	vehicle := &authorizeVehicleT{api.NewMockVehicle(ctrl), api.NewMockChargeState(ctrl)}
	vehicle.MockVehicle.EXPECT().GetTitle().Return("vehicle").AnyTimes()
	vehicle.MockVehicle.EXPECT().Features().AnyTimes()
	vehicle.MockVehicle.EXPECT().Identifiers().Return([]string{"vid"}).AnyTimes()
	// a single api roundtrip serves both detection and authorization
	vehicle.MockChargeState.EXPECT().Status().Return(api.StatusB, nil).Times(1)

	lp := newAuthorizeLoadpoint(charger, vehicle)

	// detection running, vehicle already assigned so detection only records the status match
	lp.vehicleDetect = lp.clock.Now()
	lp.vehicle = vehicle

	lp.identifyVehicleByStatus()

	charger.MockAuthorizer.EXPECT().Authorize("vid").Return(nil).Times(1)
	lp.authorizeByVehicleStatus()
}

func TestAuthorizeStatusVehicleResetOnDisconnect(t *testing.T) {
	lp := NewLoadpoint(util.NewLogger("foo"), settings.NewDatabaseSettingsAdapter("foo"))
	x, y, z := createChannels(t)
	attachChannels(lp, x, y, z)

	ctrl := gomock.NewController(t)
	lp.statusVehicle = api.NewMockVehicle(ctrl)
	lp.authorizeChecked = lp.clock.Now()

	lp.evVehicleDisconnectHandler()

	assert.Nil(t, lp.statusVehicle)
	assert.True(t, lp.authorizeChecked.IsZero())
}
