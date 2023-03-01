package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"

	paho "github.com/eclipse/paho.mqtt.golang"
	"golang.org/x/exp/slog"
	"gopkg.in/yaml.v3"
)

var (
	server    = flag.String("mqtt", "tcp://192.168.1.1:1883", "Address of the mqtt server.")
	room      = flag.String("room", "required", "Room to control")
	configDir = flag.String("config-dir", "./configs", "Directory containing the configuration files.")
	logger    = slog.New(slog.HandlerOptions{
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			if a.Key == slog.SourceKey {
				a.Value = slog.StringValue(filepath.Base(a.Value.String()))
			}
			return a
		},
	}.NewTextHandler(os.Stdout))
)

type MqttCmdTopics struct {
	Command string `yaml:"Command"`
	State   string `yaml:"State"`
}

type MqttString struct {
	MqttCmdTopics MqttCmdTopics `yaml:"MqttCmdTopics"`
	Value         string        `yaml:"Value"`
}

type MqttBool struct {
	MqttCmdTopics MqttCmdTopics `yaml:"MqttCmdTopics"`
	Value         bool          `yaml:"Value"`
}

type MqttFloat struct {
	MqttCmdTopics MqttCmdTopics `yaml:"MqttCmdTopics"`
	Value         float64       `yaml:"Value"`
}

type Hvac struct {
	AutoPilot struct {
		Enabled     MqttBool  `yaml:"Enabled"`
		Temperature MqttFloat `yaml:"Temperature"`
	} `yaml:"AutoPilot"`
	Mode   MqttString `yaml:"Mode"`
	Fan    MqttString `yaml:"Fan"`
	Device struct {
		Temperature MqttCmdTopics `yaml:"Temperature"`
		Mode        MqttCmdTopics `yaml:"Mode"`
		Fan         MqttCmdTopics `yaml:"Fan"`
	} `yaml:"Device"`
	Sensor struct {
		Value float64 `yaml:"Value"`
		Topic string  `yaml:"Topic"`
	} `yaml:"Sensor"`
}

func mustNewMqttClient() paho.Client {
	hostname, err := os.Hostname()
	if err != nil {
		logger.Error("Can't figure out the hostname", err)
		panic(err)
	}
	opts := paho.NewClientOptions()
	opts.SetClientID(fmt.Sprintf("air3-%s-%s", *room, hostname))
	opts.AddBroker(*server)
	opts.OnConnectionLost = func(client paho.Client, err error) {
		logger.Error("Lost mqtt connection", err)
		panic(err)
	}
	client := paho.NewClient(opts)
	logger.Info("Connecting to Mqtt broker.", "serveur", *server)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	return client
}

func mustLoadConfig() *Hvac {
	yfile, err := os.ReadFile(fmt.Sprintf("%s/%s.yaml", *configDir, *room))
	if err != nil {
		logger.Error("Failed to read config", err)
		panic(err)
	}
	var hvac Hvac
	err = yaml.Unmarshal(yfile, &hvac) // TODO this doesn't work
	if err != nil {
		logger.Error("Failed to parse config", err)
		panic(err)
	}
	return &hvac
}

type sensorMqttPayload struct {
	Temperature float64 `json:"temperature"`
}

func main() {
	flag.Parse()
	mqtt := mustNewMqttClient()
	hvac := mustLoadConfig()

	for topic, channel := range map[string]paho.Token{
		hvac.Sensor.Topic: mqtt.Subscribe(hvac.Sensor.Topic, 0, func(c paho.Client, m paho.Message) {
			parsed := sensorMqttPayload{}
			payload := m.Payload()
			err := json.Unmarshal(payload, &parsed)
			if err != nil {
				logger.Error("Failed to parse mqtt message", err, "topic", hvac.Sensor.Topic, "payload", payload)
				return
			}
			if math.Abs(hvac.Sensor.Value-parsed.Temperature) > 0.25 {
				logger.Info("Received update from the sensor reports.", "temperature", parsed.Temperature)
			}
			hvac.Sensor.Value = parsed.Temperature
		}),
	} {
		logger.Info("Listening to mqtt", "topic", topic)
		<-channel.Done()
		if err := channel.Error(); err != nil {
			logger.Error("Failed to subcribe to mqtt", err, "topic", topic)
		}
	}
	// Start control loop
}
