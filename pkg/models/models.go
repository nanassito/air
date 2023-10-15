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
	sensorTemp, _ := hvac.AutoPilot.Sensors.Air.Get()
	L.Info("hvac state",
		"hvac", hvac.Name,
		"autopilot.enabled", hvac.AutoPilot.Enabled.Get(),
		"autopilot.minTemp", hvac.AutoPilot.MinTemp.Get(),
		"autopilot.maxTemp", hvac.AutoPilot.MaxTemp.Get(),
		"Mode", hvac.Mode.Get(),
		"Fan", hvac.Fan.Get(),
		"TargetTemp", hvac.Temperature.Get(),
		"UnitTempRange", hvac.AutoPilot.Sensors.Unit.GetRange(),
		"SensorTempTrend", hvac.AutoPilot.Sensors.Air.GetTrend(),
		"SensorTemp", sensorTemp,
		"DecisionScore", hvac.DecisionScore,
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

func NewHvacWithDefaultTopics(mqttClient paho.Client, name string, temperatureSensorTopic string) *Hvac {
	mode_command := "esphome/" + name + "/mode_command"
	mode_state := "esphome/" + name + "/mode_state"
	fan_mode_command := "esphome/" + name + "/fan_mode_command"
	fan_mode_state := "esphome/" + name + "/fan_mode_state"
	maxTempCommand := "air3/" + name + "/autopilot/maxTemp/command"
	maxTempState := "air3/" + name + "/autopilot/maxTemp/state"
	minTempCommand := "air3/" + name + "/autopilot/minTemp/command"
	minTempState := "air3/" + name + "/autopilot/minTemp/state"
	presetCommandtopic := "air3/" + name + "/preset/command"
	presetStatetopic := "air3/" + name + "/preset/state"
	sleepMaxTemp := 22.0
	ecoMaxTemp := 33.0
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
				minTempCommand,
				minTempState,
				func(payload []byte) (float64, error) {
					return strconv.ParseFloat(string(payload), 64)
				},
				func(value float64) string {
					return strconv.FormatFloat(value, 'f', 1, 64)
				},
			),
			MaxTemp: mqtt.NewControlledValue(
				mqttClient,
				maxTempCommand,
				maxTempState,
				func(payload []byte) (float64, error) {
					temp, err := strconv.ParseFloat(string(payload), 64)
					if err == nil {
						switch int64(temp * 2) { // *2 to get rid of the floating point for .5°C
						case int64(sleepMaxTemp * 2):
							mqttClient.Publish(presetStatetopic, 0, false, "sleep")
						case int64(ecoMaxTemp * 2):
							mqttClient.Publish(presetStatetopic, 0, false, "eco")
						}
					}
					return temp, err
				},
				func(value float64) string {
					return strconv.FormatFloat(value, 'f', 1, 64)
				},
			),
			Sensors: &sensors{
				Air: mqtt.NewJsonTemperatureSensor(
					mqttClient,
					temperatureSensorTopic,
				),
				Unit: mqtt.NewRawTemperatureSensor(
					mqttClient,
					"esphome/"+name+"/current_temperature_state",
				),
			},
		},
		Mode: mqtt.NewThirdPartyValue(
			mqttClient,
			mode_command,
			mode_state,
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
			fan_mode_command,
			fan_mode_state,
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

	mqttClient.Subscribe(presetCommandtopic, 0, func(c paho.Client, m paho.Message) {
		L.Info("Received", "topic", m.Topic(), "payload", m.Payload())
		switch string(m.Payload()) {
		case "sleep":
			mqttClient.Publish(presetStatetopic, 0, false, "sleep")
			hvac.AutoPilot.MaxTemp.Set(sleepMaxTemp)
		case "eco":
			mqttClient.Publish(presetStatetopic, 0, false, "eco")
			hvac.AutoPilot.MaxTemp.Set(ecoMaxTemp)
		default:
			L.Warn("Invalid preset", "topic", m.Topic(), "payload", m.Payload())
		}
	})

	// TODO:
	// Use Mode for the autopilot-enabled, then use an icon to indicate which hvac this is about.
	mqttClient.Publish(
		"homeassistant/climate/air3/"+name+"/config",
		0,
		true,
		`{
			"name": "Thermostat",
			"max_temp": 33,
			"min_temp": 17,
			"precision": 0.5,
			"temp_step": 0.5,
			"temperature_high_command_topic": "`+maxTempCommand+`",
			"temperature_high_state_topic": "`+maxTempState+`",
			"temperature_low_command_topic": "`+minTempCommand+`",
			"temperature_low_state_topic": "`+minTempState+`",
			"current_temperature_topic": "`+temperatureSensorTopic+`",
			"current_temperature_template": "{{ value_json.temperature }}",
			"temperature_unit": "C",
			"unique_id": "`+name+`_thermostat",
			"mode_command_topic": "`+mode_command+`",
			"mode_state_topic": "`+mode_state+`",
			"fan_mode_command_topic": "`+fan_mode_command+`",
			"fan_mode_state_topic": "`+fan_mode_state+`",
			"preset_modes": ["sleep", "eco"],
			"preset_mode_command_topic": "`+presetCommandtopic+`",
			"preset_mode_state_topic": "`+presetStatetopic+`",
			"device": {
				"identifiers": "`+name+`",
				"name": "`+name+`",
				"model": "air3",
				"manufacturer": "Dorian"
			}
		}`,
	)
	return &hvac
}
