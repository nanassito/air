package models

import (
	"errors"
	"strconv"
	"strings"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"github.com/nanassito/air/pkg/mqtt"
)

var (
	ErrBadPayload = errors.New("invalid mqtt payload")
)

type sensors struct {
	Air  *mqtt.TemperatureSensor
	Unit *mqtt.TemperatureSensor
}

type autoPilot struct {
	Enabled *mqtt.ControlledValue[bool]
	MinTemp *mqtt.ControlledValue[float64]
	MaxTemp *mqtt.ControlledValue[float64]
	Sensors *sensors
}

type Hvac struct {
	Name          string
	AutoPilot     *autoPilot
	Mode          *mqtt.ThirdPartyValue[string]
	Fan           *mqtt.ThirdPartyValue[string]
	Temperature   *mqtt.ThirdPartyValue[float64]
	DecisionScore float64
	LastOff       time.Time
}

func (hvac *Hvac) Ping() {
	hvac.AutoPilot.Enabled.Set(hvac.AutoPilot.Enabled.Get())
	hvac.AutoPilot.MinTemp.Set(hvac.AutoPilot.MinTemp.Get())
	hvac.AutoPilot.MaxTemp.Set(hvac.AutoPilot.MaxTemp.Get())
}

var (
	fanSpeeds = map[string]string{
		"AUTO":   "AUTO",
		"LOW":    "LOW",
		"MEDIUM": "MEDIUM",
		"HIGH":   "HIGH",
	}
	modes = map[string]string{
		"OFF":      "OFF",
		"FAN_ONLY": "FAN_ONLY",
		"HEAT":     "HEAT",
		"COOL":     "COOL",
	}
)

func NewHvacWithDefaultTopics(mqttClient paho.Client, name string, temperatureSensor *mqtt.TemperatureSensor) *Hvac {
	hvac := Hvac{
		Name: name,
		AutoPilot: &autoPilot{
			Enabled: mqtt.NewControlledValue(
				mqttClient,
				"air3/"+name+"/autopilot/enabled/command",
				"air3/"+name+"/autopilot/enabled/state",
				func(payload []byte) (bool, error) {
					return strconv.ParseBool(string(payload))
				},
				func(value bool) string {
					return strconv.FormatBool(value)
				},
			),
			MinTemp: mqtt.NewControlledValue(
				mqttClient,
				"air3/"+name+"/autopilot/minTemp/command",
				"air3/"+name+"/autopilot/minTemp/state",
				func(payload []byte) (float64, error) {
					return strconv.ParseFloat(string(payload), 64)
				},
				func(value float64) string {
					return strconv.FormatFloat(value, 'f', 1, 64)
				},
			),
			MaxTemp: mqtt.NewControlledValue(
				mqttClient,
				"air3/"+name+"/autopilot/maxTemp/command",
				"air3/"+name+"/autopilot/maxTemp/state",
				func(payload []byte) (float64, error) {
					return strconv.ParseFloat(string(payload), 64)
				},
				func(value float64) string {
					return strconv.FormatFloat(value, 'f', 1, 64)
				},
			),
			Sensors: &sensors{
				Air: temperatureSensor,
				Unit: mqtt.NewRawTemperatureSensor(
					mqttClient,
					"esphome/"+name+"/current_temperature_state",
				),
			},
		},
		Mode: mqtt.NewThirdPartyValue(
			mqttClient,
			"esphome/"+name+"/mode_command",
			"esphome/"+name+"/mode_state",
			func(payload []byte) (string, error) {
				mode, ok := modes[strings.ToUpper(string(payload))]
				if ok {
					return mode, nil
				} else {
					return "", ErrBadPayload
				}
			},
			func(value string) string {
				return value
			},
		),
		Fan: mqtt.NewThirdPartyValue(
			mqttClient,
			"esphome/"+name+"/fan_mode_command",
			"esphome/"+name+"/fan_mode_state",
			func(payload []byte) (string, error) {
				speed, ok := fanSpeeds[strings.ToUpper(string(payload))]
				if ok {
					return speed, nil
				} else {
					return "", ErrBadPayload
				}
			},
			func(value string) string {
				return value
			},
		),
		Temperature: mqtt.NewThirdPartyValue(
			mqttClient,
			"esphome/"+name+"/target_temperature_command",
			"esphome/"+name+"/target_temperature_low_state",
			func(payload []byte) (float64, error) {
				return strconv.ParseFloat(string(payload), 64)
			},
			func(value float64) string {
				return strconv.FormatFloat(value, 'f', 1, 64)
			},
		),
		DecisionScore: 0,
	}
	return &hvac
}

func DecreaseFanSpeed(hvac *Hvac) {
	switch hvac.Fan.Get() {
	case "MEDIUM":
		hvac.Fan.Set("LOW")
	case "HIGH":
		hvac.Fan.Set("MEDIUM")
	}
}

func IncreaseFanSpeed(hvac *Hvac) {
	switch hvac.Fan.Get() {
	case "AUTO":
		hvac.Fan.Set("MEDIUM")
	case "LOW":
		hvac.Fan.Set("MEDIUM")
	case "MEDIUM":
		hvac.Fan.Set("HIGH")
	}
}
