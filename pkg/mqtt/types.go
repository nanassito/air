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
	initialized  bool
}

func (s *ThirdPartyValue[T]) IsReady() bool {
	return s.initialized
}

func (s *ThirdPartyValue[T]) Get() T {
	return s.value
}

func (s *ThirdPartyValue[T]) Set(t T) {
	s.mqtt.Publish(s.commandTopic, qos, false, s.formatter(t))

	// Check that the new value is ackenowledged and retry every 100ms for up to 1s if it isn't
	ticker := time.NewTicker(100 * time.Millisecond)
	for i := 0; i < 10; i++ {
		<-ticker.C
		if s.value == t {
			return
		} else {
			L.Warn("ThirdPartyValue was not acknowledged", "desired", t, "acknowledged", s.value, "hvac")
		}
	}
	L.Error("Failed to set a ThirdPartyValue", "desired", t, "acknowledged", s.value, "hvac")
}

func NewThirdPartyValue[T bool | string | float64](mqtt paho.Client, commandTopic string, statusTopic string, parser func([]byte) (T, error), formatter func(T) string) *ThirdPartyValue[T] {
	s := ThirdPartyValue[T]{
		mqtt:         mqtt,
		value:        *new(T),
		commandTopic: commandTopic,
		statusTopic:  statusTopic,
		parser:       parser,
		formatter:    formatter,
		initialized:  false,
	}
	s.mqtt.Subscribe(s.statusTopic, qos, func(c paho.Client, m paho.Message) {
		L.Info("Received", "topic", m.Topic(), "payload", m.Payload())
		value, err := s.parser(m.Payload())
		if err != nil {
			L.Error("Failed to parse mqtt message", "err", err, "topic", m.Topic(), "payload", m.Payload())
			return
		}
		s.value = value
		s.initialized = true
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
	initialized  bool
}

func (s *ControlledValue[T]) IsReady() bool {
	return s.initialized
}

func (s *ControlledValue[T]) Get() T {
	return s.value
}

func (s *ControlledValue[T]) Set(t T) {
	s.value = t
	s.initialized = true
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
		initialized:  false,
	}
	s.mqtt.Subscribe(s.commandTopic, qos, func(c paho.Client, m paho.Message) {
		L.Info("Received", "topic", m.Topic(), "payload", m.Payload())
		value, err := s.parser(m.Payload())
		if err != nil {
			L.Error("Failed to parse mqtt message", "err", err, "topic", m.Topic(), "payload", m.Payload())
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
	return t.values[len(t.values)-1].value, nil
}

type Trend int64

const (
	TrendWarmingUp Trend = iota
	TrendStable
	TrendCoolingDown
)

func (t *TemperatureSensor) GetTrend() Trend {
	current, err := t.GetCurrent()
	if err != nil {
		return TrendStable
	}

	min := t.values[0].value
	max := t.values[0].value
	for _, measurement := range t.values[:len(t.values)] {
		if measurement.value < min {
			min = measurement.value
		}
		if measurement.value > max {
			max = measurement.value
		}
	}
	min = min + 0.2
	max = max - 0.2

	if max > current {
		// Measurements in the past were noticeably warmer than the current measurement
		if min < current {
			// But there are also measurement noticeably cooler than the current measurement
			return TrendStable
		} else {
			return TrendCoolingDown
		}
	} else {
		if min < current {
			return TrendWarmingUp
		} else {
			return TrendStable
		}
	}
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
			L.Error("Failed to parse mqtt message", "err", err, "topic", m.Topic(), "payload", m.Payload())
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
