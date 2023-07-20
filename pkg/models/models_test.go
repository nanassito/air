package models_test

import (
	"testing"

	"github.com/matryer/is"

	"github.com/nanassito/air/pkg/mocks"
	"github.com/nanassito/air/pkg/models"
)

func TestHomeAssistantInterface(t *testing.T) {
	is := is.New(t)
	mqttClient := mocks.NewMockMqtt()
	hvac := models.NewHvacWithDefaultTopics(mqttClient, "room", nil)

	t.Run("autopilot", func(t *testing.T) {
		t.Run("on", func(t *testing.T) {
			mocks.Autopilot(mqttClient, "room", true)
			is.True(hvac.AutoPilot.Enabled.Get())
		})
		t.Run("off", func(t *testing.T) {
			mocks.Autopilot(mqttClient, "room", false)
			is.Equal(false, hvac.AutoPilot.Enabled.Get())
		})
	})

	t.Run("desired temperatures", func(t *testing.T) {
		t.Run("min", func(t *testing.T) {
			mocks.DesiredMinTemp(mqttClient, "room", 18.5)
			is.Equal(18.5, hvac.AutoPilot.MinTemp.Get())
		})
		t.Run("max", func(t *testing.T) {
			mocks.DesiredMaxTemp(mqttClient, "room", 27.0)
			is.Equal(27.0, hvac.AutoPilot.MaxTemp.Get())
		})
	})
}
