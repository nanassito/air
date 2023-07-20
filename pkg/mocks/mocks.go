package mocks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/nanassito/air/pkg/mqtt"
)

type message struct {
	topic   string
	payload []byte
}

func (m *message) Duplicate() bool {
	return false
}

func (m *message) Qos() byte {
	return 0
}

func (m *message) Retained() bool {
	return false
}

func (m *message) Topic() string {
	return m.topic
}

func (m *message) MessageID() uint16 {
	return 0
}

func (m *message) Payload() []byte {
	return m.payload
}

func (m *message) Ack() {
}

// Mocks paho.Client but with logic to match the rest of the infra.
type MockMqtt struct {
	router map[string][]paho.MessageHandler
}

func NewMockMqtt() *MockMqtt {
	return &MockMqtt{
		router: make(map[string][]paho.MessageHandler),
	}
}

func (m MockMqtt) IsConnected() bool       { return false }
func (m MockMqtt) IsConnectionOpen() bool  { return false }
func (m MockMqtt) Connect() paho.Token     { return nil }
func (m MockMqtt) Disconnect(quiesce uint) {}
func (m MockMqtt) Publish(topic string, qos byte, retained bool, payload interface{}) paho.Token {
	var data []byte
	switch p := payload.(type) {
	case string:
		data = []byte(p)
	case []byte:
		data = p
	case bytes.Buffer:
		data = p.Bytes()
	default:
		panic("invalid message type")
	}
	if callbacks, ok := m.router[topic]; ok {
		for _, callback := range callbacks {
			callback(m, &message{topic: topic, payload: data})
		}
	}
	return &paho.PublishToken{}
}
func (m MockMqtt) Subscribe(topic string, qos byte, callback paho.MessageHandler) paho.Token {
	if _, ok := m.router[topic]; !ok {
		m.router[topic] = make([]paho.MessageHandler, 0)
	}
	m.router[topic] = append(m.router[topic], callback)
	return &paho.SubscribeToken{}
}
func (m MockMqtt) SubscribeMultiple(filters map[string]byte, callback paho.MessageHandler) paho.Token {
	return &paho.SubscribeToken{}
}
func (m MockMqtt) Unsubscribe(topics ...string) paho.Token             { return nil }
func (m MockMqtt) AddRoute(topic string, callback paho.MessageHandler) {}
func (m MockMqtt) OptionsReader() paho.ClientOptionsReader             { return paho.ClientOptionsReader{} }

type MockTemperatureSensor struct {
	mqtt *MockMqtt
	name string
}

func (m *MockTemperatureSensor) Topic() string {
	return fmt.Sprintf("sensors/%s/temperature", m.name)
}

func (m *MockTemperatureSensor) Set(temp float64) {
	data, err := json.Marshal(mqtt.SensorMqttPayload{Temperature: temp})
	if err != nil {
		panic(err)
	}
	m.mqtt.Publish(m.Topic(), 0, false, data)
}

func NewMockTemperatureSensor(mockMqtt *MockMqtt, name string) *MockTemperatureSensor {
	return &MockTemperatureSensor{
		mqtt: mockMqtt,
		name: name,
	}
}

func Autopilot(mqttClient *MockMqtt, room string, enabled bool) {
	mqttClient.Publish("air3/"+room+"/autopilot/enabled/command", 0, true, strconv.FormatBool(enabled))
}

func DesiredMinTemp(mqttClient *MockMqtt, room string, temp float64) {
	mqttClient.Publish("air3/"+room+"/autopilot/minTemp/command", 0, true, strconv.FormatFloat(temp, 'f', 1, 64))
}

func DesiredMaxTemp(mqttClient *MockMqtt, room string, temp float64) {
	mqttClient.Publish("air3/"+room+"/autopilot/maxTemp/command", 0, true, strconv.FormatFloat(temp, 'f', 1, 64))
}

type MockHvac struct {
	mqtt *MockMqtt
	name string
}

func (m *MockHvac) SetDesiredTemperature(temp float64) {
	m.mqtt.Publish("esphome/"+m.name+"/mode_command", 0, false, strconv.FormatFloat(temp, 'f', 1, 64))
}

func (m *MockHvac) ReportUnitTemperature(temp float64) {
	m.mqtt.Publish("esphome/"+m.name+"/current_temperature_state", 0, false, strconv.FormatFloat(temp, 'f', 1, 64))
}

func (m *MockHvac) SetMode(mode string) {
	m.mqtt.Publish("esphome/"+m.name+"/mode_command", 0, false, mode)
}

func (m *MockHvac) SetFan(fan string) {
	m.mqtt.Publish("esphome/"+m.name+"/fan_mode_command", 0, false, fan)
}

func NewMockHvac(mockMqtt *MockMqtt, name string) *MockHvac {
	// Proxy the command to the state as if the unit was perfect.
	mockMqtt.Subscribe("esphome/"+name+"/mode_command", 0, func(c paho.Client, m paho.Message) {
		// TODO: Assert known good modes
		mockMqtt.Publish("esphome/"+name+"/mode_state", 0, false, m.Payload())
	})
	mockMqtt.Subscribe("esphome/"+name+"/fan_mode_command", 0, func(c paho.Client, m paho.Message) {
		// TODO: Assert known good fan modes
		mockMqtt.Publish("esphome/"+name+"/fan_mode_state", 0, false, m.Payload())
	})
	mockMqtt.Subscribe("esphome/"+name+"/target_temperature_command", 0, func(c paho.Client, m paho.Message) {
		// TODO: Assert valid temperature
		mockMqtt.Publish("esphome/"+name+"/target_temperature_low_state", 0, false, m.Payload())
	})
	hvac := MockHvac{
		mqtt: mockMqtt,
		name: name,
	}
	hvac.SetMode("OFF")
	return &hvac
}
