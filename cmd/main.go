package main

import (
	"flag"
	"time"

	"github.com/nanassito/air/pkg/logic"
	"github.com/nanassito/air/pkg/models"
	"github.com/nanassito/air/pkg/mqtt"
	"github.com/nanassito/air/pkg/utils"
)

var (
	server = flag.String("mqtt", "tcp://192.168.1.1:1883", "Address of the mqtt server.")
	L      = utils.Logger
)

func main() {
	flag.Parse()
	mqttClient := mqtt.MustNewMqttClient(*server)

	hvacs := []*models.Hvac{
		models.NewHvacWithDefaultTopics(
			mqttClient,
			"office",
			mqtt.NewJsonTemperatureSensor(
				mqttClient,
				"zigbee2mqtt/server/device/office/followme",
			),
		),
		models.NewHvacWithDefaultTopics(
			mqttClient,
			"living",
			mqtt.NewJsonTemperatureSensor(
				mqttClient,
				"zigbee2mqtt/server/device/living/followme",
			),
		),
		models.NewHvacWithDefaultTopics(
			mqttClient,
			"kitchen",
			mqtt.NewJsonTemperatureSensor(
				mqttClient,
				"zigbee2mqtt/server/device/kitchen/followme",
			),
		),
		models.NewHvacWithDefaultTopics(
			mqttClient,
			"parent",
			mqtt.NewJsonTemperatureSensor(
				mqttClient,
				"zigbee2mqtt/server/device/parent/followme",
			),
		),
		models.NewHvacWithDefaultTopics(
			mqttClient,
			"zaya",
			mqtt.NewJsonTemperatureSensor(
				mqttClient,
				"zigbee2mqtt/raspi/device/zaya/air",
			),
		),
	}
	for range time.Tick(30 * time.Second) {
		L.Info("Autopilot run.")
		for _, hvac := range hvacs {
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
			if hvac.AutoPilot.Enabled.Get() {
				L.Info("Autopilot is enabled on this hvac", "hvac", hvac.Name)
				switch hvac.Mode.Get() {
				case "HEAT":
					logic.TuneHeat(hvac)
				// case "COOL":
				// 	logic.TuneCold(hvac)
				case "OFF":
					logic.StartHeat(hvac)
				}
			} else {
				L.Info("Autopilot is disabled on this hvac", "hvac", hvac.Name)
			}
			hvac.Ping()
		}
	}
}
