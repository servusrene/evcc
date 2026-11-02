package ocpp

import (
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	directionSend = "send"
	directionRecv = "recv"

	messageLogSize     = 200 // messages kept per station
	messageLogStations = 32  // bounds memory against arbitrary connecting station ids
)

var authKeyRe = regexp.MustCompile(`("key":"AuthorizationKey"[^}]*?"value":")[^"]*`)

// Message is a raw OCPP frame exchanged with a station
type Message struct {
	Time      time.Time `json:"time"`
	Direction string    `json:"direction"` // send or recv
	Frame     string    `json:"frame"`
}

// messages records the latest OCPP frames per station at transport level, independent of the log level
var messages = newMessageLog()

// messageLog records the latest OCPP frames per station
type messageLog struct {
	mu       sync.Mutex
	stations map[string][]Message
}

func newMessageLog() *messageLog {
	return &messageLog{stations: make(map[string][]Message)}
}

// add records a frame sent to or received from a station
func (l *messageLog) add(direction, id string, data []byte) {
	if l == nil {
		return
	}

	frame := authKeyRe.ReplaceAllString(strings.TrimSpace(string(data)), "${1}***")

	l.mu.Lock()
	defer l.mu.Unlock()

	msgs, ok := l.stations[id]
	if !ok && len(l.stations) >= messageLogStations {
		return
	}

	msgs = append(msgs, Message{Time: time.Now(), Direction: direction, Frame: frame})
	if len(msgs) > messageLogSize {
		msgs = msgs[len(msgs)-messageLogSize:]
	}
	l.stations[id] = msgs
}

// get returns a copy of the recorded messages of a station, oldest first
func (l *messageLog) get(id string) []Message {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]Message{}, l.stations[id]...)
}
