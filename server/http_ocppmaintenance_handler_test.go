package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/charger/ocpp"
	"github.com/stretchr/testify/assert"
)

func TestOcppMaintenanceError(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{fmt.Errorf("%w: foo", ocpp.ErrUnknownStation), http.StatusNotFound},
		{ocpp.ErrNoTransaction, http.StatusConflict},
		{api.ErrTimeout, http.StatusGatewayTimeout},
		{errors.New("Rejected"), http.StatusUnprocessableEntity},
	} {
		w := httptest.NewRecorder()
		ocppMaintenanceError(w, tc.err)
		assert.Equal(t, tc.status, w.Code, tc.err.Error())
	}
}
