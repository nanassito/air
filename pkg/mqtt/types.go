package mqtt

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

var (
	qos                  = byte(0)
	ErrNotInitializedYet = errors.New("not initialized yet")
)

type valueWithHistory[T comparable] struct {
	MaxAge   time.Duration
	timeData map[time.Time]T
	latest   time.Time
}

func (s *valueWithHistory[T]) Insert(newValue T) {
	if value, ok := s.timeData[s.latest]; ok && value == newValue {
		return // Value is unchanged
	}
	timeData := make(map[time.Time]T, len(s.timeData))
	for when, value := range s.timeData {
		if time.Since(when) > s.MaxAge || when == s.latest {
			timeData[when] = value
		}
	}
	now := time.Now()
	timeData[now] = newValue
	s.latest = now
	s.timeData = timeData
}

type ThirdPartyValue[T bool | string | float64] struct {
	mqtt         paho.Client
	values       *valueWithHistory[T]
	commandTopic string
	statusTopic  string
	parser       func([]byte) (T, error)
	formatter    func(T) string
}

func (s *ThirdPartyValue[T]) IsReady() bool {
	return len(s.values.timeData) >= 1
}

func (s *ThirdPartyValue[T]) Get() T {
	return s.values.timeData[s.values.latest]
}

func (s *ThirdPartyValue[T]) UnchangedFor() time.Duration {
	if len(s.values.timeData) > 1 {
		return time.Since(s.values.latest)
	} else {
		return 24 * time.Hour // Just something large enough since we don't really know
	}
}

func (s *ThirdPartyValue[T]) Set(t T) {
	rs := s.mqtt.Publish(s.commandTopic, qos, false, s.formatter(t))
	rs.Wait()
	if err := rs.Error(); err != nil {
		L.Error("mqtt error", "err", err, "commandTopic", s.commandTopic)
		return
	}

	// Check that the new value is acknowledged and retry every 100ms for up to 1s if it isn't
	ticker := time.NewTicker(300 * time.Millisecond)
	for i := 0; i < 10; i++ {
		<-ticker.C
		if s.IsReady() && s.Get() == t {
			return
		} else {
			L.Warn("ThirdPartyValue was not acknowledged", "desired", t, "acknowledged", s.Get(), "statusTopic", s.statusTopic)
		}
	}
	L.Error("Failed to set a ThirdPartyValue", "desired", t, "acknowledged", s.Get(), "statusTopic", s.statusTopic)
}

func NewThirdPartyValue[T bool | string | float64](mqtt paho.Client, commandTopic string, statusTopic string, parser func([]byte) (T, error), formatter func(T) string) *ThirdPartyValue[T] {
	s := ThirdPartyValue[T]{
		mqtt:         mqtt,
		values:       &valueWithHistory[T]{MaxAge: 1 * time.Hour},
		commandTopic: commandTopic,
		statusTopic:  statusTopic,
		parser:       parser,
		formatter:    formatter,
	}
	s.mqtt.Subscribe(s.statusTopic, qos, func(c paho.Client, m paho.Message) {
		L.Info("Received", "topic", m.Topic(), "payload", m.Payload())
		value, err := s.parser(m.Payload())
		if err != nil {
			L.Error("Failed to parse mqtt message", "err", err, "topic", m.Topic(), "payload", m.Payload())
			return
		}
		s.values.Insert(value)
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

type TemperatureSensor struct {
	values *valueWithHistory[float64]
}

type SensorMqttPayload struct {
	Temperature float64 `json:"temperature"`
}

func (t *TemperatureSensor) Get() (float64, error) {
	if len(t.values.timeData) == 0 {
		return 0, ErrNotInitializedYet
	}
	return t.values.timeData[t.values.latest], nil
}

type Trend int64

const (
	TrendWarmingUp Trend = iota
	TrendStable
	TrendCoolingDown
)

func (t *TemperatureSensor) GetTrend() Trend {
	current, err := t.Get()
	if err != nil {
		return TrendStable
	}

	min := current
	max := current
	for ts, measurement := range t.values.timeData {
		if ts == t.values.latest {
			continue
		}
		if measurement < min {
			min = measurement
		}
		if measurement > max {
			max = measurement
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

func NewJsonTemperatureSensor(mqtt paho.Client, topic string) *TemperatureSensor {
	t := TemperatureSensor{
		values: &valueWithHistory[float64]{MaxAge: 1 * time.Hour},
	}
	mqtt.Subscribe(topic, qos, func(c paho.Client, m paho.Message) {
		L.Info("Received", "topic", m.Topic(), "payload", m.Payload())
		parsed := SensorMqttPayload{}
		err := json.Unmarshal(m.Payload(), &parsed)
		if err != nil {
			L.Error("Failed to parse mqtt message", "err", err, "topic", m.Topic(), "payload", m.Payload())
			return
		}
		t.values.Insert(parsed.Temperature)
	})
	return &t
}

func NewRawTemperatureSensor(mqtt paho.Client, topic string) *TemperatureSensor {
	t := TemperatureSensor{
		values: &valueWithHistory[float64]{MaxAge: 1 * time.Hour},
	}
	mqtt.Subscribe(topic, qos, func(c paho.Client, m paho.Message) {
		L.Info("Received", "topic", m.Topic(), "payload", m.Payload())
		value, err := strconv.ParseFloat(string(m.Payload()), 64)
		if err != nil {
			L.Error("Failed to parse mqtt message", "err", err, "topic", m.Topic(), "payload", m.Payload())
			return
		}
		t.values.Insert(value)
	})
	return &t
}
