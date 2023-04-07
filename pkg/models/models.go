package models

import (
	"errors"
	"strconv"
	"strings"

	paho "github.com/eclipse/paho.mqtt.golang"

	"github.com/nanassito/air/pkg/mqtt"
)

var (
	ErrBadPayload = errors.New("invalid mqtt payload")
)

type autoPilot struct {
	Enabled *mqtt.ControlledValue[bool]
	MinTemp *mqtt.ControlledValue[float64]
	Sensor  *mqtt.TemperatureSensor
}

type Hvac struct {
	Name          string
	AutoPilot     *autoPilot
	Mode          *mqtt.ThirdPartyValue[string]
	Fan           *mqtt.ThirdPartyValue[string]
	Temperature   *mqtt.ThirdPartyValue[float64]
	DecisionScore float64
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
					value, ok := map[string]bool{
						"ON":  true,
						"OFF": false,
					}[strings.ToUpper(string(payload))]
					if ok {
						return value, nil
					} else {
						return *new(bool), ErrBadPayload
					}
				},
				func(value bool) string {
					return map[bool]string{
						true:  "on",
						false: "off",
					}[value]
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
			Sensor: temperatureSensor,
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
