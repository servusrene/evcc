package ocpp

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageLog(t *testing.T) {
	l := newMessageLog()

	l.add("send", "st-1", []byte(`[2,"1","Reset",{"type":"Soft"}]`))
	l.add("recv", "st-1", []byte(`[3,"1",{"status":"Accepted"}]`))
	l.add("recv", "st-2", []byte(`[2,"2","Heartbeat",{}]`))

	msgs := l.get("st-1")
	require.Len(t, msgs, 2)
	assert.Equal(t, "send", msgs[0].Direction)
	assert.Equal(t, `[2,"1","Reset",{"type":"Soft"}]`, msgs[0].Frame)
	assert.Equal(t, "recv", msgs[1].Direction)
	assert.Len(t, l.get("st-2"), 1)
	assert.Empty(t, l.get("unknown"))

	// bounded per station
	for i := range messageLogSize + 10 {
		l.add("recv", "st-3", fmt.Appendf(nil, `[2,"%d","Heartbeat",{}]`, i))
	}
	msgs = l.get("st-3")
	require.Len(t, msgs, messageLogSize)
	assert.Equal(t, `[2,"10","Heartbeat",{}]`, msgs[0].Frame)

	// bounded number of stations
	for i := range messageLogStations {
		l.add("recv", fmt.Sprintf("extra-%d", i), []byte(`[]`))
	}
	assert.Len(t, l.stations, messageLogStations)
	assert.Len(t, l.get("st-1"), 2)
}

func TestMessageLogRedactsAuthorizationKey(t *testing.T) {
	l := newMessageLog()

	l.add("send", "st", []byte(`[2,"1","ChangeConfiguration",{"key":"AuthorizationKey","value":"secret"}]`))
	l.add("recv", "st", []byte(`[3,"2",{"configurationKey":[{"key":"AuthorizationKey","readonly":false,"value":"secret"},{"key":"HeartbeatInterval","value":"60"}]}]`))

	for _, m := range l.get("st") {
		assert.NotContains(t, m.Frame, "secret")
		assert.Contains(t, m.Frame, `"AuthorizationKey"`)
	}
	assert.Contains(t, l.get("st")[1].Frame, `"value":"60"`)
}

func TestMessageLogNil(t *testing.T) {
	var l *messageLog
	l.add("send", "st", []byte(`[]`))
	assert.Nil(t, l.get("st"))
}
