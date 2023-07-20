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

	pumps := []*models.Pump{
		{
			Units: []*models.Hvac{
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
			},
		},
		{
			Units: []*models.Hvac{
				models.NewHvacWithDefaultTopics(
					mqttClient,
					"living",
					mqtt.NewJsonTemperatureSensor(
						mqttClient,
						"zigbee2mqtt/server/device/living/followme",
					),
				),
			},
		},
	}
	for range time.Tick(30 * time.Second) {
		L.Info("Autopilot run.")
		for _, pump := range pumps {
			logic.TunePump(pump)
		}
	}
}
