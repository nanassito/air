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

func TuneHeat(hvac *models.Hvac, pump *models.Pump) {
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
	L.Info("Tuning heat", "current", current, "minDesired", hvac.AutoPilot.MaxTemp.Get(), "hvac", hvac.Name)

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
		hvac.DecisionScore = 0
		if hvac.Temperature.Get() <= getAvgTargetTemp(pump) && hvac.Fan.Get() != "AUTO" && hvac.Fan.Get() != "LOW" {
			L.Info("Reducing fan speed", "hvac", hvac.Name)
			hvac.DecreaseFanSpeed()
		} else {
			L.Info("Reducing fan temperature", "hvac", hvac.Name)
			hvac.Temperature.Set(hvac.Temperature.Get() - 0.5)

		}
	case 100:
		hvac.DecisionScore = 0
		if hvac.Temperature.Get()-getAvgTargetTemp(pump) > 3 && hvac.Fan.Get() != "HIGH" {
			// There is too much imbalance between this hvac and the others, we need to mix the air more.
			L.Info("Too much imbalance, increasing fan speed", "hvac", hvac.Name)
			hvac.IncreaseFanSpeed()
			hvac.Temperature.Set(hvac.Temperature.Get() - 0.5)
		} else {
			L.Info("Increasing temperature", "hvac", hvac.Name)
			hvac.Temperature.Set(hvac.Temperature.Get() + 0.5)
		}
	}
	L.Info("Completing TuneHeat", "hvac", hvac.Name, "decisionScore", hvac.DecisionScore)
}

func getAvgTargetTemp(pump *models.Pump) float64 {
	var total float64
	for _, hvac := range pump.Units {
		total += hvac.Temperature.Get()
	}
	return total / float64(len(pump.Units))
}
