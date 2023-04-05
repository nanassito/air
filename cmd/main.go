package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"gopkg.in/yaml.v3"

	"github.com/nanassito/air/pkg/autopilot"
	"github.com/nanassito/air/pkg/listeners"
	"github.com/nanassito/air/pkg/models"
	"github.com/nanassito/air/pkg/utils"
)

var (
	server    = flag.String("mqtt", "tcp://192.168.1.1:1883", "Address of the mqtt server.")
	room      = flag.String("room", "required", "Room to control")
	configDir = flag.String("config-dir", "./configs", "Directory containing the configuration files.")
	L         = utils.Logger
)

func mustNewMqttClient() paho.Client {
	hostname, err := os.Hostname()
	if err != nil {
		L.Error("Can't figure out the hostname", err)
		panic(err)
	}
	opts := paho.NewClientOptions()
	opts.SetClientID(fmt.Sprintf("air3-%s-%s", *room, hostname))
	opts.AddBroker(*server)
	opts.OnConnectionLost = func(client paho.Client, err error) {
		L.Error("Lost mqtt connection", err)
		panic(err)
	}
	client := paho.NewClient(opts)
	L.Info("Connecting to Mqtt broker.", "serveur", *server)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	return client
}

func mustLoadConfig() *models.Hvac {
	yfile, err := os.ReadFile(fmt.Sprintf("%s/%s.yaml", *configDir, *room))
	if err != nil {
		L.Error("Failed to read config", err)
		panic(err)
	}
	var hvac models.Hvac
	err = yaml.Unmarshal(yfile, &hvac) // TODO this doesn't work
	if err != nil {
		L.Error("Failed to parse config", err)
		panic(err)
	}
	return &hvac
}

type sensorMqttPayload struct {
	Temperature float64 `json:"temperature"`
}

func readSensor(hvac *models.Hvac) paho.MessageHandler {
	return func(c paho.Client, m paho.Message) {
		L.Info("Received update from sensor.", "payload", m.Payload())
		parsed := sensorMqttPayload{}
		err := json.Unmarshal(m.Payload(), &parsed)
		if err != nil {
			L.Error("Failed to parse mqtt message", err, "topic", m.Topic(), "payload", m.Payload())
			return
		}
		if math.Abs(hvac.Sensor.Value-parsed.Temperature) > 0.25 {
			L.Info("Received update from the sensor reports.", "temperature", parsed.Temperature)
		}
		hvac.Sensor.Value = parsed.Temperature
	}
}

func checkAndUpdate(hvac *models.Hvac, mqtt paho.Client) {
	updates := map[string]paho.Token{}

	if hvac.Frontend.Fan.Value != hvac.Backend.Fan.Value && hvac.Backend.Mode.Value != "OFF" && hvac.Backend.Mode.Value != "" {
		L.Info("Adjusting device fan speed.", "from", hvac.Backend.Fan.Value, "to", hvac.Frontend.Fan.Value)
		speed, ok := map[int64]string{
			0: "AUTO",
			1: "LOW",
			2: "MEDIUM",
			3: "HIGH",
		}[hvac.Frontend.Fan.Value]
		if !ok {
			L.Warn("WTF? Invalid fan speed.", "hvac.Fan.Value", hvac.Frontend.Fan.Value)
			return
		}
		updates[hvac.Backend.Fan.MqttCmdTopics.Command] = mqtt.Publish(hvac.Backend.Fan.MqttCmdTopics.Command, listeners.Qos, false, speed)
	}

	for topic, token := range updates {
		<-token.Done()
		if err := token.Error(); err != nil {
			L.Error("Mqtt.send error", err, "topic", topic)
		}
	}
}

func main() {
	flag.Parse()
	mqtt := mustNewMqttClient()
	hvac := mustLoadConfig()

	tokens := map[string]paho.Token{}
	for topic, callback := range map[string]paho.MessageHandler{
		hvac.Sensor.Topic:                               readSensor(hvac),
		hvac.Frontend.Fan.MqttCmdTopics.Command:         listeners.ReadFanCommand(hvac),
		hvac.Backend.Fan.MqttCmdTopics.State:            listeners.ReadFanState(hvac, mqtt),
		hvac.Frontend.Mode.MqttCmdTopics.Command:        listeners.ReadModeCommand(hvac),
		hvac.Backend.Mode.MqttCmdTopics.State:           listeners.ReadModeState(hvac, mqtt),
		hvac.Frontend.Temperature.MqttCmdTopics.Command: listeners.ReadTargetTemperatureCommand(hvac),
		hvac.Backend.Temperature.MqttCmdTopics.State:    listeners.ReadTargetTemperatureState(hvac, mqtt),
	} {
		L.Info("Listening to mqtt", "topic", topic)
		tokens[topic] = mqtt.Subscribe(hvac.Sensor.Topic, listeners.Qos, callback)
	}
	for topic, token := range tokens {
		<-token.Done()
		if err := token.Error(); err != nil {
			L.Error("Failed to subcribe to mqtt", err, "topic", topic)
		}
	}

	go func() {
		for range time.Tick(5 * time.Minute) {
			autopilot.RunOnce(hvac)
		}
	}()

	// Start control loop
	for range time.Tick(100 * time.Millisecond) {
		checkAndUpdate(hvac, mqtt)
	}
}
