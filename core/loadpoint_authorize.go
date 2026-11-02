package core

import (
	"crypto/subtle"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/core/keys"
	"github.com/evcc-io/evcc/util"
)

const (
	authorizeMaxFailures = 5
	authorizeLockout     = 5 * time.Minute
)

var (
	ErrAuthorizeNotSupported = errors.New("charger does not support authorization")
	ErrAuthorizeNotWaiting   = errors.New("charger is not waiting for authorization")
	ErrAuthorizeInvalidPin   = errors.New("invalid pin")
	ErrAuthorizeLocked       = errors.New("too many failed attempts")

	authorizePinRegex = regexp.MustCompile(`^[0-9]{4}$`)
)

// GetAuthorizePin returns the pin required for manual charger authorization
func (lp *Loadpoint) GetAuthorizePin() string {
	lp.RLock()
	defer lp.RUnlock()
	return lp.authorizePin
}

// SetAuthorizePin sets the pin required for manual charger authorization, empty disables it
func (lp *Loadpoint) SetAuthorizePin(pin string) error {
	if pin != "" && !authorizePinRegex.MatchString(pin) {
		return errors.New("pin must have 4 digits")
	}

	lp.Lock()
	defer lp.Unlock()

	if pin != lp.authorizePin {
		lp.setAuthorizePin(pin)
	}

	return nil
}

// setAuthorizePin sets the pin (no mutex)
func (lp *Loadpoint) setAuthorizePin(pin string) {
	lp.authorizePin = pin
	lp.settings.SetString(keys.AuthorizePin, pin)
	// never publish the pin itself
	lp.publish(keys.AuthorizePinRequired, pin != "")
}

// waitingForAuthorization checks if the charger reports a pending authorization
func (lp *Loadpoint) waitingForAuthorization() bool {
	sr, ok := api.Cap[api.StatusReasoner](lp.charger)
	if !ok || lp.GetStatus() != api.StatusB {
		return false
	}

	r, err := sr.StatusReason()
	return err == nil && r == api.ReasonWaitingForAuthorization
}

// Authorize authorizes the charging session on behalf of the user, verifying the configured pin
func (lp *Loadpoint) Authorize(pin string) error {
	authorizer, ok := api.Cap[api.Authorizer](lp.charger)
	if !ok {
		return ErrAuthorizeNotSupported
	}

	lp.Lock()
	if lp.clock.Now().Before(lp.authorizeLocked) {
		lp.Unlock()
		return ErrAuthorizeLocked
	}

	if lp.authorizePin != "" && subtle.ConstantTimeCompare([]byte(pin), []byte(lp.authorizePin)) != 1 {
		lp.authorizeFailures++
		if lp.authorizeFailures >= authorizeMaxFailures {
			lp.log.WARN.Printf("authorize: %d failed attempts, locked for %v", lp.authorizeFailures, authorizeLockout)
			lp.authorizeFailures = 0
			lp.authorizeLocked = lp.clock.Now().Add(authorizeLockout)
		}
		lp.Unlock()
		return ErrAuthorizeInvalidPin
	}

	lp.authorizeFailures = 0
	lp.Unlock()

	if !lp.waitingForAuthorization() {
		return ErrAuthorizeNotWaiting
	}

	lp.log.INFO.Println("authorize: manual")

	return authorizer.Authorize("")
}

// authorizeByVehicleStatus authorizes the charger once the vehicle api reports the vehicle as connected.
// Default or manually selected vehicles deliberately do not authorize.
func (lp *Loadpoint) authorizeByVehicleStatus() {
	authorizer, ok := api.Cap[api.Authorizer](lp.charger)
	if !ok || len(lp.coordinatedVehicles()) == 0 || !lp.waitingForAuthorization() {
		return
	}

	// limit vehicle api polling to the detection window, afterwards only manual authorization remains
	if lp.clock.Since(lp.connectedTime) > vehicleDetectDuration {
		return
	}

	if !lp.authorizeChecked.IsZero() && lp.clock.Since(lp.authorizeChecked) < vehicleDetectInterval {
		return
	}
	lp.authorizeChecked = lp.clock.Now()

	// reuse the result of vehicle detection to avoid a second api roundtrip
	vehicle := lp.statusVehicle
	if vehicle == nil {
		// running vehicle detection already refreshes the api cache
		if lp.vehicleDetect.IsZero() {
			util.ResetCached()
		}

		if vehicle = lp.coordinator.IdentifyVehicleByStatus(); vehicle == nil {
			return
		}
		lp.statusVehicle = vehicle
	}

	lp.log.INFO.Printf("authorize: vehicle %s connected", vehicle.GetTitle())

	if err := authorizer.Authorize(vehicleAuthorizeKey(vehicle)); err != nil {
		lp.log.ERROR.Printf("authorize: %v", err)
	}
}

// vehicleAuthorizeKey returns the first identifier usable as charger id tag
func vehicleAuthorizeKey(v api.Vehicle) string {
	ids := v.Identifiers()
	if i := slices.IndexFunc(ids, func(id string) bool {
		return id != "" && !strings.Contains(id, "*")
	}); i >= 0 {
		return ids[i]
	}
	return ""
}
