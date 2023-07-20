package models

import (
	"errors"
	"strconv"
	"strings"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang-collections/collections/set"

	"github.com/nanassito/air/pkg/mqtt"
	"github.com/nanassito/air/pkg/utils"
)

var (
	ErrBadPayload = errors.New("invalid mqtt payload")
	L             = utils.Logger
	fanSpeeds     = map[string]string{
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

type Pump struct {
	Units []*Hvac
}

func (pump *Pump) GetUsableModes() *set.Set {
	usableModes := set.New()
	for _, mode := range modes {
		usableModes.Insert(mode)
	}
	for _, hvac := range pump.Units {
		usableModes = usableModes.Intersection(set.New("OFF", hvac.Mode.Get()))
	}
	if usableModes.Has("OFF") && usableModes.Len() == 1 {
		return set.New("OFF", "HEAT", "COOL", "FAN_ONLY")
	}
	L.Info("", "usableModes", usableModes)
	return usableModes
}

type Hvac struct {
	Name          string
	AutoPilot     *autoPilot
	Mode          *mqtt.ThirdPartyValue[string]
	Fan           *mqtt.ThirdPartyValue[string]
	Temperature   *mqtt.ThirdPartyValue[float64]
	DecisionScore float64
}

func (hvac *Hvac) Log() {
	L.Info("hvac state",
		"hvac", hvac.Name,
		"autopilot.enabled", hvac.AutoPilot.Enabled.Get(),
		"autopilot.minTemp", hvac.AutoPilot.MinTemp.Get(),
		"autopilot.maxTemp", hvac.AutoPilot.MaxTemp.Get(),
		"Mode", hvac.Mode.Get(),
		"Fan", hvac.Fan.Get(),
		"Temperature", hvac.Temperature.Get(),
		"decisionScore", hvac.DecisionScore,
	)
}

func (hvac *Hvac) DecreaseFanSpeed() {
	switch hvac.Fan.Get() {
	case "MEDIUM":
		hvac.Fan.Set("LOW")
	case "HIGH":
		hvac.Fan.Set("MEDIUM")
	}
}

func (hvac *Hvac) IncreaseFanSpeed() {
	switch hvac.Fan.Get() {
	case "AUTO":
		hvac.Fan.Set("MEDIUM")
	case "LOW":
		hvac.Fan.Set("MEDIUM")
	case "MEDIUM":
		hvac.Fan.Set("HIGH")
	}
}

func (hvac *Hvac) Ping() {
	hvac.AutoPilot.Enabled.Set(hvac.AutoPilot.Enabled.Get())
	hvac.AutoPilot.MinTemp.Set(hvac.AutoPilot.MinTemp.Get())
	hvac.AutoPilot.MaxTemp.Set(hvac.AutoPilot.MaxTemp.Get())
}

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
