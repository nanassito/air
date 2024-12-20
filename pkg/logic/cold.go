package logic

import (
	"math"
	"time"

	"github.com/nanassito/air/pkg/models"
	"github.com/nanassito/air/pkg/mqtt"
)

func StartCold(hvac *models.Hvac) {
	if hvac.Mode.UnchangedFor() < 30*time.Minute {
		L.Error("Hvac mode changed recently, preventing flapping.")
		return
	}

	current, err := getCurrentTemp(hvac)
	if err != nil {
		L.Error(err.Error(), "hvac", hvac.Name)
		return
	}
	if !hvac.AutoPilot.MaxTemp.IsReady() {
		L.Error("autopilot max temperature isn't initialized yet.", "hvac", hvac.Name)
		return
	}

	if current >= hvac.AutoPilot.MaxTemp.Get()-1 {
		L.Info("Temperature rised enough that we should restart the cooling cycle.", "hvac", hvac.Name)
		hvac.DecisionScore = 0
		inUnit, err := hvac.AutoPilot.Sensors.Unit.Get()
		if err != nil {
			L.Info("unknown current temperature in the unit", "hvac", hvac.Name)
			return
		}
		hvac.Mode.Set("COOL")

		if inUnit > hvac.AutoPilot.MaxTemp.Get()+2 {
			// If there is a large temperature difference between the in-unit sensor and the target temperature,
			// we want to first mix the air.
			hvac.Temperature.Set(30)
			hvac.Fan.Set("HIGH")
			go func() {
				// After some time we can tweak the settings to maximize comfort.
				time.Sleep(5 * time.Minute)
				hvac.Fan.Set("AUTO")
				inUnit, err := hvac.AutoPilot.Sensors.Unit.Get()
				if err != nil {
					L.Info("unknown current temperature in the unit", "hvac", hvac.Name)
					return
				}
				hvac.Temperature.Set(math.Max(inUnit, hvac.AutoPilot.MaxTemp.Get()+2))
			}()
		} else {
			// The HVAC unit has a flawed perception of the temperature in the room and so it can't set it's own
			// temperature correctly. We make up for it by targetting teh higher of the in-unit temperature and
			// the desired temperature (plus a buffer) to minimize the risk of over-cooling.
			hvac.Temperature.Set(math.Max(inUnit, hvac.AutoPilot.MaxTemp.Get()+2))
			hvac.Fan.Set("AUTO")
		}
	}
}

func TuneCold(hvac *models.Hvac, pump *models.Pump) {
	current, err := getCurrentTemp(hvac)
	if err != nil {
		L.Error(err.Error(), "hvac", hvac.Name)
		return
	}
	if !hvac.AutoPilot.MaxTemp.IsReady() {
		L.Error("autopilot max temperature isn't initialized yet.", "hvac", hvac.Name)
		return
	}

	maxDesired := hvac.AutoPilot.MaxTemp.Get()
	L.Info("Tuning cold", "current", current, "maxDesired", hvac.AutoPilot.MaxTemp.Get(), "hvac", hvac.Name)

	if current < maxDesired-3 {
		L.Info("It's way too cold, shutting down", "hvac", hvac.Name)
		hvac.Mode.Set("OFF")
		hvac.DecisionScore = 0
		return
	}
	unitTempRange := hvac.AutoPilot.Sensors.Unit.GetRange()
	if hvac.Mode.UnchangedFor() > 3*time.Hour && current < maxDesired && unitTempRange < 1 && hvac.AutoPilot.Sensors.Air.GetTrend() != mqtt.TrendWarmingUp {
		L.Info("Unit hasn't been effective for a while, shutting down", "hvac", hvac.Name, "unitTempRange", unitTempRange)
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
		minOffset = -0.5

	case mqtt.TrendWarmingUp:
		L.Info("Trend is warming up", "hvac", hvac.Name)
		minOffset = 0.5

	default:
		L.Warn("Unknown trend", "trend", hvac.AutoPilot.Sensors.Air.GetTrend(), "hvac", hvac.Name)
		minOffset = 0
	}

	if current < maxDesired-1+minOffset {
		hvac.DecisionScore += 1
		L.Info("Need less cold", "hvac", hvac.Name)
	} else if current >= maxDesired+minOffset {
		hvac.DecisionScore -= 1
		L.Info("Need more cold", "hvac", hvac.Name)
	} else {
		L.Info("Not doing anything", "hvac", hvac.Name)
	}

	switch hvac.DecisionScore {
	case -60:
		L.Info("Reducing temperature", "hvac", hvac.Name)
		hvac.DecisionScore = 0
		hvac.Temperature.Set(hvac.Temperature.Get() - 0.5)
	case 60:
		L.Info("Increasing temperature", "hvac", hvac.Name)
		hvac.DecisionScore = 0
		hvac.Temperature.Set(hvac.Temperature.Get() + 0.5)
	}
	L.Info("Completing TuneCold", "hvac", hvac.Name, "decisionScore", hvac.DecisionScore)
}
