package models

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

type MqttInt struct {
	MqttCmdTopics MqttCmdTopics `yaml:"MqttCmdTopics"`
	Value         int64         `yaml:"Value"`
}

type TempModeFan struct {
	Temperature MqttFloat  `yaml:"Temperature"`
	Mode        MqttString `yaml:"Mode"`
	Fan         MqttInt    `yaml:"Fan"`
}

type Hvac struct {
	AutoPilot struct {
		Enabled     MqttBool  `yaml:"Enabled"`
		Temperature MqttFloat `yaml:"Temperature"`
	} `yaml:"AutoPilot"`
	Frontend TempModeFan `yaml:"Frontend"` // What we want the hvac to do.
	Backend  TempModeFan `yaml:"Backend"`  // What the hvac is trying to do.
	Sensor   struct {
		Value float64 `yaml:"Value"`
		Topic string  `yaml:"Topic"`
	} `yaml:"Sensor"`
}
