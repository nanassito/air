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

	hvacs := []models.Hvac{
		*models.NewHvacWithDefaultTopics(
			mqttClient,
			"office",
			mqtt.NewTemperatureSensor(
				mqttClient,
				"zigbee2mqtt/server/device/office/followme",
			),
		),
	}
	for range time.Tick(5 * time.Minute) {
		L.Info("Autopilot run.")
		for _, hvac := range hvacs {
			if hvac.AutoPilot.Enabled.Get() {
				L.Info("Autopilot is enabled on this hvac", "hvac", hvac.Name)
				logic.TuneHeat(&hvac)
			} else {
				L.Info("Autopilot is disabled on this hvac", "hvac", hvac.Name)
			}
		}
	}
}
