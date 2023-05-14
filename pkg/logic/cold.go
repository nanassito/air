package logic

import (
	"math"

	"github.com/nanassito/air/pkg/models"
	"github.com/nanassito/air/pkg/mqtt"
)

func TuneCold(hvac *models.Hvac) {
	current, err := hvac.AutoPilot.Sensor.GetCurrent()
	if err != nil {
		L.Error("Don't have a current temperature from the sensor yet.", "hvac", hvac.Name)
		return
	}
	if !hvac.AutoPilot.MaxTemp.IsReady() {
		L.Error("autopilot max temperature isn't initialized yet.", "hvac", hvac.Name)
		return
	}

	L.Info("Current temperature", "t", current, "hvac", hvac.Name)

	if hvac.Mode.Get() == "COOL" && current < hvac.AutoPilot.MaxTemp.Get()-3 {
		L.Info("It's way too cold, shutting down", "hvac", hvac.Name)
		hvac.Mode.Set("OFF")
		hvac.DecisionScore = 0
		return
	}

	if hvac.Mode.Get() == "OFF" && current >= hvac.AutoPilot.MaxTemp.Get()-1 {
		L.Info("Temperature warmed enough that we should restart the cooling cycle.", "hvac", hvac.Name)
		hvac.DecisionScore = 0
		hvac.Mode.Set("COOL")
		hvac.Fan.Set("AUTO")
		hvac.Temperature.Set(math.Max(hvac.Temperature.Get()-3, hvac.AutoPilot.MaxTemp.Get()))
		return
	}

	maxOffset := 0.0
	switch hvac.AutoPilot.Sensor.GetTrend() {
	case mqtt.TrendStable:
		L.Info("Trend is stable", "hvac", hvac.Name)
		maxOffset = 0

	case mqtt.TrendCoolingDown:
		L.Info("Trend is cooling down", "hvac", hvac.Name)
		maxOffset = 0.5

	case mqtt.TrendWarmingUp:
		L.Info("Trend is warming up", "hvac", hvac.Name)
		maxOffset = -0.5

	default:
		L.Warn("Unknown trend", "trend", hvac.AutoPilot.Sensor.GetTrend(), "hvac", hvac.Name)
		maxOffset = 0
	}

	if current >= hvac.AutoPilot.MaxTemp.Get()+maxOffset {
		hvac.DecisionScore -= 1
		L.Info("Need more cold", "hvac", hvac.Name)
	} else if current < hvac.AutoPilot.MinTemp.Get()-1+maxOffset {
		hvac.DecisionScore += 1
		L.Info("Need less cold", "hvac", hvac.Name)
	} else {
		L.Info("Not doing anything", "hvac", hvac.Name)
	}

	switch hvac.DecisionScore {
	case -100:
		L.Info("Increasing fan speed and increasing temperature", "hvac", hvac.Name)
		hvac.DecisionScore = 0
		models.DecreaseFanSpeed(hvac)
		hvac.Temperature.Set(hvac.Temperature.Get() + 0.5)
	case -50:
		L.Info("Increasing temperature", "hvac", hvac.Name)
		hvac.Temperature.Set(hvac.Temperature.Get() + 0.5)
	case 50:
		L.Info("Reducing temperature", "hvac", hvac.Name)
		hvac.Temperature.Set(hvac.Temperature.Get() - 0.5)
	case 100:
		L.Info("Reducing fan speed and lowering temperature", "hvac", hvac.Name)
		hvac.DecisionScore = 0
		models.IncreaseFanSpeed(hvac)
		hvac.Temperature.Set(hvac.Temperature.Get() - 0.5)
	}
	L.Info("Completing TuneCold", "hvac", hvac.Name, "decisionScore", hvac.DecisionScore)
}
