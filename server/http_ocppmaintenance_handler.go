package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/charger/ocpp"
	"github.com/gorilla/mux"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
)

// ocppMaintenanceError maps maintenance errors to http status codes
func ocppMaintenanceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ocpp.ErrUnknownStation):
		jsonError(w, http.StatusNotFound, err)
	case errors.Is(err, ocpp.ErrNoTransaction):
		jsonError(w, http.StatusConflict, err)
	case errors.Is(err, api.ErrTimeout):
		jsonError(w, http.StatusGatewayTimeout, err)
	default:
		// charger rejected the request
		jsonError(w, http.StatusUnprocessableEntity, err)
	}
}

// ocppStationHandler returns the maintenance view of a configured OCPP station
func ocppStationHandler(w http.ResponseWriter, r *http.Request) {
	res, err := ocpp.GetStationInfo(mux.Vars(r)["id"])
	if err != nil {
		ocppMaintenanceError(w, err)
		return
	}

	jsonWrite(w, res)
}

// ocppResetHandler sends a soft or hard reset to a configured OCPP station
func ocppResetHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	resetType := core.ResetTypeSoft
	if vars["type"] == "hard" {
		resetType = core.ResetTypeHard
	}

	if err := ocpp.ResetStation(vars["id"], resetType); err != nil {
		ocppMaintenanceError(w, err)
		return
	}

	jsonWrite(w, true)
}

// ocppConnectorHandler runs a maintenance action on a connector of a configured OCPP station
func ocppConnectorHandler(action func(string, int) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		connector, err := strconv.Atoi(vars["connector"])
		if err != nil {
			jsonError(w, http.StatusBadRequest, err)
			return
		}

		if err := action(vars["id"], connector); err != nil {
			ocppMaintenanceError(w, err)
			return
		}

		jsonWrite(w, true)
	}
}

// ocppStationLogHandler returns the latest OCPP frames of a configured OCPP station
func ocppStationLogHandler(w http.ResponseWriter, r *http.Request) {
	res, err := ocpp.GetStationMessages(mux.Vars(r)["id"])
	if err != nil {
		ocppMaintenanceError(w, err)
		return
	}

	jsonWrite(w, res)
}
