package main

import (
	"bytes"
	"log"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

type IoTExporterAuthHook struct {
	mqtt.HookBase
}

// ID returns the ID of the hook.
func (h *IoTExporterAuthHook) ID() string {
	return "iot-exporter-auth"
}

// Provides indicates which hook methods this hook provides.
func (h *IoTExporterAuthHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnectAuthenticate,
		mqtt.OnACLCheck,
	}, []byte{b})
}

// OnConnectAuthenticate returns true/allowed for all requests.
func (h *IoTExporterAuthHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	return true
}

// OnACLCheck returns true/allowed for all checks.
func (h *IoTExporterAuthHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	deviceIdParsed := deviceIdFromTopicRegex.FindStringSubmatch(topic)
	if deviceIdParsed == nil {
		return false
	}

	deviceIdFromTopic := deviceIdParsed[1]
	if deviceIdFromTopic != string(cl.Properties.Username) {
		log.Printf("client %s tried to access topic %s, but is only allowed to access topics for device %s", cl.ID, topic, string(cl.Properties.Username))
		return false
	}

	return true
}
