package logic

import (
	"time"

	"github.com/nanassito/air/pkg/models"
	"github.com/nanassito/air/pkg/mqtt"
)

func StartHeat(hvac *models.Hvac) {
	if hvac.Mode.UnchangedFor() < 30*time.Minute {
		L.Error("Hvac mode changed recently, preventing flapping.")
		return
	}

	current, err := getCurrentTemp(hvac)
	if err != nil {
		L.Error(err.Error(), "hvac", hvac.Name)
		return
	}
	if !hvac.AutoPilot.MinTemp.IsReady() {
		L.Error("autopilot min temperature isn't initialized yet.", "hvac", hvac.Name)
		return
	}

	if current <= hvac.AutoPilot.MinTemp.Get()+1 {
		if hvac.Mode.UnchangedFor() < 30*time.Minute {
			L.Info("Hvac was shutdown not long enough ago.", "hvac", hvac.Name)
			return
		}
		L.Info("Temperature lowered enough that we should restart the heating cycle.", "hvac", hvac.Name)
		hvac.DecisionScore = 0
		hvac.Mode.Set("HEAT")
		hvac.Fan.Set("AUTO")
		hvac.Temperature.Set(hvac.AutoPilot.MinTemp.Get())
		return
	}
}

func TuneHeat(hvac *models.Hvac) {
	current, err := getCurrentTemp(hvac)
	if err != nil {
		L.Error(err.Error(), "hvac", hvac.Name)
		return
	}
	if !hvac.AutoPilot.MinTemp.IsReady() {
		L.Error("autopilot min temperature isn't initialized yet.", "hvac", hvac.Name)
		return
	}

	minDesired := hvac.AutoPilot.MinTemp.Get()
	L.Info("Tuning heat", "current", current, "minDesired", hvac.AutoPilot.MaxTemp.Get())

	if current > minDesired+3 {
		L.Info("It's way too hot, shutting down", "hvac", hvac.Name)
		hvac.Mode.Set("OFF")
		hvac.DecisionScore = 0
		return
	}

	minOffset := 0.0
	switch hvac.AutoPilot.Sensors.Air.GetTrend() {
	case mqtt.TrendStable:
		L.Info("Trend is stable", "hvac", hvac.Name)
		minOffset = 0

	case mqtt.TrendCoolingDown:
		L.Info("Trend is cooling down", "hvac", hvac.Name)
		minOffset = 0.5

	case mqtt.TrendWarmingUp:
		L.Info("Trend is warming up", "hvac", hvac.Name)
		minOffset = -0.5

	default:
		L.Warn("Unknown trend", "trend", hvac.AutoPilot.Sensors.Air.GetTrend(), "hvac", hvac.Name)
		minOffset = 0
	}

	if current <= minDesired+minOffset {
		hvac.DecisionScore += 1
		L.Info("Need more heat", "hvac", hvac.Name)
	} else if current > minDesired+1+minOffset {
		hvac.DecisionScore -= 1
		L.Info("Need less heat", "hvac", hvac.Name)
	} else {
		L.Info("Not doing anything", "hvac", hvac.Name)
	}

	switch hvac.DecisionScore {
	case -100:
		if hvac.Temperature.Get() == 17.0 {
			L.Info("Heating is ineffective, shutting down", "hvac", hvac.Name)
			hvac.Mode.Set("OFF")
			hvac.DecisionScore = 0
			return
		}
		L.Info("Reducing fan speed and temperature", "hvac", hvac.Name)
		hvac.DecisionScore = 0
		hvac.DecreaseFanSpeed()
		hvac.Temperature.Set(hvac.Temperature.Get() - 0.5)
	case -50:
		L.Info("Reducing temperature", "hvac", hvac.Name)
		hvac.Temperature.Set(hvac.Temperature.Get() - 0.5)
	case 50:
		L.Info("Increasing temperature", "hvac", hvac.Name)
		hvac.Temperature.Set(hvac.Temperature.Get() + 0.5)
	case 100:
		L.Info("Increasing fan speed and temperature", "hvac", hvac.Name)
		hvac.DecisionScore = 0
		hvac.IncreaseFanSpeed()
		hvac.Temperature.Set(hvac.Temperature.Get() + 0.5)
	}
	L.Info("Completing TuneHeat", "hvac", hvac.Name, "decisionScore", hvac.DecisionScore)
}
