package listeners

import (
	"strconv"

	paho "github.com/eclipse/paho.mqtt.golang"

	"github.com/nanassito/air/pkg/models"
	"github.com/nanassito/air/pkg/utils"
)

var (
	L   = utils.Logger
	Qos = byte(0)
)

func ReadFanCommand(hvac *models.Hvac) paho.MessageHandler {
	return func(c paho.Client, m paho.Message) {
		L.Info("Received fan speed command.", "payload", m.Payload())
		speed, err := strconv.ParseInt(string(m.Payload()), 10, 64)
		if err != nil {
			L.Error("Failed to parse fan speed command.", err, "topic", m.Topic(), "payload", m.Payload())
			return
		}
		if 0 <= speed && speed <= 3 {
			hvac.Frontend.Fan.Value = speed
		} else {
			L.Warn("Received invalid fan speed command.", "speed", speed)
		}
	}
}

func ReadFanState(hvac *models.Hvac, mqtt paho.Client) paho.MessageHandler {
	return func(c paho.Client, m paho.Message) {
		L.Info("Received a fan speed update from the device.", "payload", m.Payload())
		speed, ok := map[string]int64{
			"AUTO":   0,
			"LOW":    1,
			"MEDIUM": 2,
			"HIGH":   3,
		}[string(m.Payload())]
		if !ok {
			L.Warn("Received an invalid fan speed update.", "topic", m.Topic(), "payload", m.Payload())
			return
		}
		hvac.Backend.Fan.Value = speed
		token := mqtt.Publish(hvac.Frontend.Fan.MqttCmdTopics.State, Qos, false, speed)
		<-token.Done()
		if err := token.Error(); err != nil {
			L.Error("Mqtt.send error", err, "topic", hvac.Frontend.Fan.MqttCmdTopics.State)
		}
	}
}

func ReadModeCommand(hvac *models.Hvac) paho.MessageHandler {
	return func(c paho.Client, m paho.Message) {
		L.Info("Received mode command.", "payload", m.Payload())
		mode := string(m.Payload())
		if _, ok := map[string]struct{}{
			"OFF":      {},
			"FAN_ONLY": {},
			"COOL":     {},
			"HEAT":     {},
			"AUTO":     {},
			"DRY":      {},
		}[mode]; !ok {
			L.Warn("Failed to parse mode command.", "topic", m.Topic(), "payload", m.Payload())
			return
		}
		hvac.Frontend.Mode.Value = mode
	}
}

func ReadModeState(hvac *models.Hvac, mqtt paho.Client) paho.MessageHandler {
	return func(c paho.Client, m paho.Message) {
		L.Info("Received a mode update from the device.", "payload", m.Payload())
		hvac.Backend.Mode.Value = string(m.Payload())
		token := mqtt.Publish(hvac.Frontend.Mode.MqttCmdTopics.State, Qos, false, string(m.Payload()))
		<-token.Done()
		if err := token.Error(); err != nil {
			L.Error("Mqtt.send error", err, "topic", hvac.Frontend.Mode.MqttCmdTopics.State)
		}
	}
}

func ReadTargetTemperatureCommand(hvac *models.Hvac) paho.MessageHandler {
	return func(c paho.Client, m paho.Message) {
		L.Info("Received target temperature command.", "payload", m.Payload())
		temp, err := strconv.ParseFloat(string(m.Payload()), 64)
		if err != nil {
			L.Error("Failed to parse target temperature command.", err, "topic", m.Topic(), "payload", m.Payload())
			return
		}
		hvac.Frontend.Temperature.Value = temp
	}
}

func ReadTargetTemperatureState(hvac *models.Hvac, mqtt paho.Client) paho.MessageHandler {
	return func(c paho.Client, m paho.Message) {
		L.Info("Received a mode update from the device.", "payload", m.Payload())
		temp, err := strconv.ParseFloat(string(m.Payload()), 64)
		if err != nil {
			L.Error("Failed to parse target temperature state.", err, "topic", m.Topic(), "payload", m.Payload())
			return
		}
		hvac.Backend.Temperature.Value = temp
		token := mqtt.Publish(hvac.Frontend.Temperature.MqttCmdTopics.State, Qos, false, string(m.Payload()))
		<-token.Done()
		if err := token.Error(); err != nil {
			L.Error("Mqtt.send error", err, "topic", hvac.Frontend.Temperature.MqttCmdTopics.State)
		}
	}
}
