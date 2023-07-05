package logic

import (
	"errors"

	"github.com/nanassito/air/pkg/models"
	"github.com/nanassito/air/pkg/utils"
)

var L = utils.Logger

func getCurrentTemp(hvac *models.Hvac) (float64, error) {
	current, err := hvac.AutoPilot.Sensors.Air.GetCurrent()
	if err != nil {
		return 0, errors.New("don't have a current temperature from the sensor yet")
	}
	if !hvac.AutoPilot.MinTemp.IsReady() {
		L.Error("autopilot min temperature isn't initialized yet.", "hvac", hvac.Name)
		return 0, errors.New("autopilot min temperature isn't initialized yet")
	}

	L.Info("Current temperature", "t", current, "hvac", hvac.Name)
	return current, nil
}
