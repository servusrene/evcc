package ocpp

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
)

var (
	// ErrUnknownStation is returned for charge points not configured in evcc
	ErrUnknownStation = errors.New("unknown station")
	// ErrNoTransaction is returned when stopping a connector without a known transaction
	ErrNoTransaction = errors.New("no known transaction")
)

// configured returns the configured charge point for maintenance requests
func (cs *CS) configured(id string) (*CP, error) {
	cp, err := cs.ChargepointByID(id)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownStation, id)
	}
	return cp, nil
}

// ConnectorInfo is the maintenance view of a single connector
type ConnectorInfo struct {
	ID            int                       `json:"id"`
	Status        core.ChargePointStatus    `json:"status,omitempty"`
	ErrorCode     core.ChargePointErrorCode `json:"errorCode,omitempty"`
	Info          string                    `json:"info,omitempty"`
	TransactionID int                       `json:"transactionId,omitempty"`
	IdTag         string                    `json:"idTag,omitempty"`
}

// StationInfo is the maintenance view of a charge point
type StationInfo struct {
	ID              string          `json:"id"`
	Status          StationStatus   `json:"status"`
	Vendor          string          `json:"vendor,omitempty"`
	Model           string          `json:"model,omitempty"`
	SerialNumber    string          `json:"serialNumber,omitempty"`
	FirmwareVersion string          `json:"firmwareVersion,omitempty"`
	Connectors      []ConnectorInfo `json:"connectors"`
}

// bootInfo returns a copy of the last BootNotification
func (cp *CP) bootInfo() *core.BootNotificationRequest {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	if cp.BootNotificationResult == nil {
		return nil
	}

	res := *cp.BootNotificationResult
	return &res
}

// connectorIDs returns the ids of all connectors configured in evcc
func (cp *CP) connectorIDs() []int {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	return slices.Collect(maps.Keys(cp.connectors))
}

// stationInfo returns the maintenance view of a configured charge point
func (cs *CS) stationInfo(id string) (StationInfo, error) {
	cp, err := cs.configured(id)
	if err != nil {
		return StationInfo{}, err
	}

	res := StationInfo{
		ID:     id,
		Status: StationStatusConfigured,
	}

	if cp.Connected() {
		res.Status = StationStatusConnected
	}

	if boot := cp.bootInfo(); boot != nil {
		res.Vendor = boot.ChargePointVendor
		res.Model = boot.ChargePointModel
		res.SerialNumber = boot.ChargePointSerialNumber
		res.FirmwareVersion = boot.FirmwareVersion
	}

	connectors := make(map[int]*ConnectorInfo)
	for _, cid := range cp.connectorIDs() {
		connectors[cid] = &ConnectorInfo{ID: cid}
	}

	// cached status notifications include connectors not configured in evcc
	cs.mu.Lock()
	if reg, ok := cs.regs[id]; ok {
		reg.mu.RLock()
		for cid, status := range reg.status {
			if cid == 0 || status == nil {
				continue
			}
			if connectors[cid] == nil {
				connectors[cid] = &ConnectorInfo{ID: cid}
			}
			connectors[cid].Status = status.Status
			connectors[cid].ErrorCode = status.ErrorCode
			connectors[cid].Info = status.Info
		}
		reg.mu.RUnlock()
	}
	cs.mu.Unlock()

	for cid, info := range connectors {
		if conn := cp.connectorByID(cid); conn != nil {
			conn.mu.Lock()
			info.TransactionID = conn.txnId
			info.IdTag = conn.idTag
			conn.mu.Unlock()
		}
	}

	res.Connectors = make([]ConnectorInfo, 0, len(connectors))
	for _, cid := range slices.Sorted(maps.Keys(connectors)) {
		info := *connectors[cid]
		if info.ErrorCode == core.NoError {
			info.ErrorCode = ""
		}
		res.Connectors = append(res.Connectors, info)
	}

	return res, nil
}

// resetStation sends a soft or hard reset to a configured charge point
func (cs *CS) resetStation(id string, resetType core.ResetType) error {
	if resetType != core.ResetTypeSoft && resetType != core.ResetTypeHard {
		return fmt.Errorf("invalid reset type: %s", resetType)
	}

	cp, err := cs.configured(id)
	if err != nil {
		return err
	}

	return cp.ResetRequest(resetType)
}

// stopTransaction requests the charge point to stop the transaction known for the connector
func (cs *CS) stopTransaction(id string, connectorId int) error {
	cp, err := cs.configured(id)
	if err != nil {
		return err
	}

	conn := cp.connectorByID(connectorId)
	if conn == nil {
		return ErrNoTransaction
	}

	txnId, err := conn.TransactionID()
	if err != nil {
		return err
	}
	if txnId == 0 {
		return ErrNoTransaction
	}

	return cp.RemoteStopTransactionRequest(txnId)
}

// unlockConnector requests the charge point to unlock the connector
func (cs *CS) unlockConnector(id string, connectorId int) error {
	if connectorId < 1 {
		return fmt.Errorf("invalid connector: %d", connectorId)
	}

	cp, err := cs.configured(id)
	if err != nil {
		return err
	}

	return cp.UnlockConnectorRequest(connectorId)
}

// running returns the central system without starting it
func running() (*CS, error) {
	if instance == nil {
		return nil, ErrUnknownStation
	}
	return instance, nil
}

// GetStationInfo returns the maintenance view of a configured charge point
func GetStationInfo(id string) (StationInfo, error) {
	cs, err := running()
	if err != nil {
		return StationInfo{}, err
	}
	return cs.stationInfo(id)
}

// ResetStation sends a soft or hard reset to a configured charge point
func ResetStation(id string, resetType core.ResetType) error {
	cs, err := running()
	if err != nil {
		return err
	}
	return cs.resetStation(id, resetType)
}

// StopTransaction requests a configured charge point to stop the transaction known for the connector
func StopTransaction(id string, connectorId int) error {
	cs, err := running()
	if err != nil {
		return err
	}
	return cs.stopTransaction(id, connectorId)
}

// UnlockConnector requests a configured charge point to unlock the connector
func UnlockConnector(id string, connectorId int) error {
	cs, err := running()
	if err != nil {
		return err
	}
	return cs.unlockConnector(id, connectorId)
}

// GetStationMessages returns the latest OCPP frames of a configured charge point
func GetStationMessages(id string) ([]Message, error) {
	cs, err := running()
	if err != nil {
		return nil, err
	}
	if _, err := cs.configured(id); err != nil {
		return nil, err
	}
	return messages.get(id), nil
}
