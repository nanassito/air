package logic

import (
	"errors"

	"github.com/nanassito/air/pkg/models"
	"github.com/nanassito/air/pkg/mqtt"
	"github.com/nanassito/air/pkg/utils"
)

var L = utils.Logger

func TuneHeat(hvac *models.Hvac) {
	current, err := hvac.AutoPilot.Sensor.GetCurrent()
	if err != nil {
		L.Error("Don't have a current temperature from the sensor yet.", err, "hvac", hvac.Name)
		return
	}
	if !hvac.AutoPilot.MinTemp.IsReady() {
		L.Error("autopilot min temperature isn't initialized yet.", errors.New(""), "hvac", hvac.Name)
		return
	}

	if current > hvac.AutoPilot.MinTemp.Get()+3 {
		L.Info("It's way too hot, shutting down", "hvac", hvac.Name)
		hvac.Mode.Set("OFF")
		hvac.DecisionScore = 0
		return
	}

	if hvac.Mode.Get() == "OFF" && current <= hvac.AutoPilot.MinTemp.Get()+1 {
		// Temperature lowered enough that we should restart the heating cycle
		hvac.DecisionScore = 0
		hvac.Mode.Set("HEAT")
		hvac.Fan.Set("AUTO")
		hvac.Temperature.Set(hvac.AutoPilot.MinTemp.Get())
		return
	}

	switch hvac.AutoPilot.Sensor.GetTrend() {
	case mqtt.TrendStable:
		L.Info("Trend is stable", "hvac", hvac.Name)
		if current <= hvac.AutoPilot.MinTemp.Get() {
			hvac.DecisionScore += 1
		}
		if current > hvac.AutoPilot.MinTemp.Get()+1 {
			hvac.DecisionScore -= 1
		}

	case mqtt.TrendCoolingDown:
		L.Info("Trend is cooling down", "hvac", hvac.Name)
		if current <= hvac.AutoPilot.MinTemp.Get()+0.5 {
			hvac.DecisionScore += 1
		}
		if current > hvac.AutoPilot.MinTemp.Get()+1+0.5 {
			hvac.DecisionScore -= 1
		}

	case mqtt.TrendWarmingUp:
		L.Info("Trend is warming up", "hvac", hvac.Name)
		if current <= hvac.AutoPilot.MinTemp.Get()-0.5 {
			hvac.DecisionScore += 1
		}
		if current > hvac.AutoPilot.MinTemp.Get()+1-0.5 {
			hvac.DecisionScore -= 1
		}

	default:
		L.Warn("Unknown trend", "trend", hvac.AutoPilot.Sensor.GetTrend(), "hvac", hvac.Name)
	}

	switch hvac.DecisionScore {
	case -100:
		hvac.DecisionScore = 0
		models.DecreaseFanSpeed(hvac)
		hvac.Temperature.Set(hvac.Temperature.Get() - 0.5)
	case -50:
		hvac.Temperature.Set(hvac.Temperature.Get() - 0.5)
	case 50:
		hvac.Temperature.Set(hvac.Temperature.Get() + 0.5)
	case 100:
		hvac.DecisionScore = 0
		models.IncreaseFanSpeed(hvac)
		hvac.Temperature.Set(hvac.Temperature.Get() + 0.5)
	}
	L.Info("Completing TuneHeat", "hvac", hvac.Name, "decisionScore", hvac.DecisionScore)
}
