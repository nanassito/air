package mqtt

import (
	"encoding/json"
	"errors"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

var (
	qos                  = byte(0)
	ErrNotInitializedYet = errors.New("not initialized yet")
)

type ThirdPartyValue[T bool | string | float64] struct {
	mqtt         paho.Client
	value        T
	commandTopic string
	statusTopic  string
	parser       func([]byte) (T, error)
	formatter    func(T) string
}

func (s *ThirdPartyValue[T]) Get() T {
	return s.value
}

func (s *ThirdPartyValue[T]) Set(t T) {
	s.mqtt.Publish(s.commandTopic, qos, false, s.formatter(t))
}

func NewThirdPartyValue[T bool | string | float64](mqtt paho.Client, commandTopic string, statusTopic string, parser func([]byte) (T, error), formatter func(T) string) *ThirdPartyValue[T] {
	s := ThirdPartyValue[T]{
		mqtt:         mqtt,
		value:        *new(T),
		commandTopic: commandTopic,
		statusTopic:  statusTopic,
		parser:       parser,
		formatter:    formatter,
	}
	s.mqtt.Subscribe(s.statusTopic, qos, func(c paho.Client, m paho.Message) {
		L.Info("Received", "topic", m.Topic(), "payload", m.Payload())
		value, err := s.parser(m.Payload())
		if err != nil {
			L.Error("Failed to parse mqtt message", err, "topic", m.Topic(), "payload", m.Payload())
			return
		}
		s.value = value
	})
	return &s
}

type ControlledValue[T bool | string | float64] struct {
	mqtt         paho.Client
	value        T
	commandTopic string
	statusTopic  string
	parser       func([]byte) (T, error)
	formatter    func(T) string
}

func (s *ControlledValue[T]) Get() T {
	return s.value
}

func (s *ControlledValue[T]) Set(t T) {
	s.value = t
	s.mqtt.Publish(s.statusTopic, qos, false, s.formatter(t))
}

func NewControlledValue[T bool | string | float64](mqtt paho.Client, commandTopic string, statusTopic string, parser func([]byte) (T, error), formatter func(T) string) *ControlledValue[T] {
	s := ControlledValue[T]{
		mqtt:         mqtt,
		value:        *new(T),
		commandTopic: commandTopic,
		statusTopic:  statusTopic,
		parser:       parser,
		formatter:    formatter,
	}
	s.mqtt.Subscribe(s.commandTopic, qos, func(c paho.Client, m paho.Message) {
		L.Info("Received", "topic", m.Topic(), "payload", m.Payload())
		value, err := s.parser(m.Payload())
		if err != nil {
			L.Error("Failed to parse mqtt message", err, "topic", m.Topic(), "payload", m.Payload())
			return
		}
		s.Set(value)
	})
	return &s
}

type sensorRecord struct {
	when  time.Time
	value float64
}

type TemperatureSensor struct {
	values []sensorRecord
}

type sensorMqttPayload struct {
	Temperature float64 `json:"temperature"`
}

func (t *TemperatureSensor) GetCurrent() (float64, error) {
	if len(t.values) == 0 {
		return 0, ErrNotInitializedYet
	}
	return t.values[len(t.values)].value, nil
}

func NewTemperatureSensor(mqtt paho.Client, topic string) *TemperatureSensor {
	t := TemperatureSensor{
		values: make([]sensorRecord, 0),
	}
	mqtt.Subscribe(topic, qos, func(c paho.Client, m paho.Message) {
		L.Info("Received", "topic", m.Topic(), "payload", m.Payload())
		parsed := sensorMqttPayload{}
		err := json.Unmarshal(m.Payload(), &parsed)
		if err != nil {
			L.Error("Failed to parse mqtt message", err, "topic", m.Topic(), "payload", m.Payload())
			return
		}
		t.values = append(t.values, struct {
			when  time.Time
			value float64
		}{when: time.Now(), value: parsed.Temperature})
		for i, v := range t.values {
			if v.when.Before(time.Now().Add(-1 * time.Hour)) { // Prune data older than 1h
				continue
			}
			// t.values is ordered by timestamp so we know all remaining data should be kept.
			t.values = t.values[i:]
			break
		}
	})
	return &t
}
