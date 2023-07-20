package logic_test

import (
	"testing"

	"github.com/matryer/is"

	"github.com/nanassito/air/pkg/logic"
	"github.com/nanassito/air/pkg/mocks"
	"github.com/nanassito/air/pkg/models"
	"github.com/nanassito/air/pkg/mqtt"
)

func TestHeatTurnsOn(t *testing.T) {
	is := is.New(t)
	mqttClient := mocks.NewMockMqtt()

	roomName := "test_room"
	roomTemp := mocks.NewMockTemperatureSensor(mqttClient, "sensor1")
	pumps := []*models.Pump{
		{
			Units: []*models.Hvac{
				models.NewHvacWithDefaultTopics(
					mqttClient,
					roomName,
					mqtt.NewJsonTemperatureSensor(
						mqttClient,
						roomTemp.Topic(),
					),
				),
			},
		},
	}
	mocks.NewMockHvac(mqttClient, roomName)

	mocks.Autopilot(mqttClient, roomName, true)
	mocks.DesiredMinTemp(mqttClient, roomName, 20)
	roomTemp.Set(18)

	logic.TunePump(pumps[0])

	is.Equal("HEAT", pumps[0].Units[0].Mode.Get())
}

func TestColdTurnsOn(t *testing.T) {
	is := is.New(t)
	mqttClient := mocks.NewMockMqtt()

	roomName := "test_room"
	roomTemp := mocks.NewMockTemperatureSensor(mqttClient, "sensor1")
	pumps := []*models.Pump{
		{
			Units: []*models.Hvac{
				models.NewHvacWithDefaultTopics(
					mqttClient,
					roomName,
					mqtt.NewJsonTemperatureSensor(
						mqttClient,
						roomTemp.Topic(),
					),
				),
			},
		},
	}
	hvac := mocks.NewMockHvac(mqttClient, roomName)
	hvac.ReportUnitTemperature(26)

	mocks.Autopilot(mqttClient, roomName, true)
	mocks.DesiredMaxTemp(mqttClient, roomName, 23)
	roomTemp.Set(28)

	logic.TunePump(pumps[0])

	is.Equal("COOL", pumps[0].Units[0].Mode.Get())
}

func TestDontFlipMode(t *testing.T) {

	is := is.New(t)
	mqttClient := mocks.NewMockMqtt()

	roomName := "test_room"
	roomTemp := mocks.NewMockTemperatureSensor(mqttClient, "sensor1")
	pumps := []*models.Pump{
		{
			Units: []*models.Hvac{
				models.NewHvacWithDefaultTopics(
					mqttClient,
					roomName,
					mqtt.NewJsonTemperatureSensor(
						mqttClient,
						roomTemp.Topic(),
					),
				),
			},
		},
	}
	hvac := mocks.NewMockHvac(mqttClient, roomName)
	hvac.ReportUnitTemperature(26)

	mocks.Autopilot(mqttClient, roomName, true)
	mocks.DesiredMaxTemp(mqttClient, roomName, 23)
	mocks.DesiredMinTemp(mqttClient, roomName, 20)
	roomTemp.Set(28)

	logic.TunePump(pumps[0])

	is.Equal("COOL", pumps[0].Units[0].Mode.Get())

	roomTemp.Set(18) // The AC cooled off the room too much

	logic.TunePump(pumps[0])

	is.Equal("OFF", pumps[0].Units[0].Mode.Get()) // OFF Because we are far from the target
}
