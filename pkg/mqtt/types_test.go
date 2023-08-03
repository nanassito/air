package mqtt_test

import (
	"strconv"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/matryer/is"
	"github.com/nanassito/air/pkg/mocks"
	"github.com/nanassito/air/pkg/mqtt"
)

func Test3rdPartyValue(t *testing.T) {
	is := is.New(t)
	mockMqtt := mocks.NewMockMqtt()

	mockMqtt.Subscribe("command", 0, func(c paho.Client, m paho.Message) {
		mockMqtt.Publish("status", 0, false, m.Payload())
	})

	v := mqtt.NewThirdPartyValue(
		mockMqtt,
		"command",
		"status",
		func(payload []byte) (bool, error) { return strconv.ParseBool(string(payload)) },
		func(value bool) string { return strconv.FormatBool(value) },
	)

	v.Set(true)
	is.True(v.IsReady())
	is.True(v.Get())

	v.Set(false)
	v.Set(false)
	is.Equal(false, v.Get())

	is.True(v.UnchangedFor() < 1*time.Second)
}

func TestGetRange(t *testing.T) {
	is := is.New(t)
	mockMqtt := mocks.NewMockMqtt()

	mockMqtt.Subscribe("command", 0, func(c paho.Client, m paho.Message) {
		mockMqtt.Publish("status", 0, false, m.Payload())
	})

	s := mqtt.NewRawTemperatureSensor(mockMqtt, "topic")
	mockMqtt.Publish("topic", 0, false, "24")
	mockMqtt.Publish("topic", 0, false, "24") // A second time to check the dedup works.

	is.Equal(0.0, s.GetRange()) // We have a single value so the range should be 0

	mockMqtt.Publish("topic", 0, false, "24.5")
	is.Equal(0.5, s.GetRange())
}
