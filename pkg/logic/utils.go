package logic

import (
	"errors"

	"github.com/nanassito/air/pkg/models"
	"github.com/nanassito/air/pkg/utils"
)

var L = utils.Logger

func getCurrentTemp(hvac *models.Hvac) (float64, error) {
	current, err := hvac.AutoPilot.Sensors.Air.Get()
	if err != nil {
		return 0, errors.New("don't have a current temperature from the sensor yet")
	}
	L.Info("Current temperature", "t", current, "hvac", hvac.Name)
	return current, nil
}

func TunePump(pump *models.Pump) {
	usableModes := pump.GetUsableModes()
	for _, hvac := range pump.Units {
		hvac.Log()
		if hvac.AutoPilot.Enabled.Get() {
			L.Info("Autopilot is enabled on this hvac", "hvac", hvac.Name)
			if usableModes.Has("HEAT") {
				if hvac.Mode.Get() == "OFF" {
					StartHeat(hvac)
				}
				if hvac.Mode.Get() == "HEAT" {
					TuneHeat(hvac)
				}
			}
			if usableModes.Has("COOL") {
				if hvac.Mode.Get() == "OFF" {
					StartCold(hvac)
				}
				if hvac.Mode.Get() == "COOL" {
					TuneCold(hvac)
				}
			}
		} else {
			L.Info("Autopilot is disabled on this hvac", "hvac", hvac.Name)
		}
		hvac.Ping()
	}
}
