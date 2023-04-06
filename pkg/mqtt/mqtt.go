package mqtt

import (
	"fmt"
	"os"

	paho "github.com/eclipse/paho.mqtt.golang"

	"github.com/nanassito/air/pkg/utils"
)

var L = utils.Logger

func MustNewMqttClient(server string) paho.Client {
	hostname, err := os.Hostname()
	if err != nil {
		L.Error("Can't figure out the hostname", err)
		panic(err)
	}
	opts := paho.NewClientOptions()
	opts.SetClientID(fmt.Sprintf("air3-%s", hostname))
	opts.AddBroker(server)
	opts.OnConnectionLost = func(client paho.Client, err error) {
		L.Error("Lost mqtt connection", err)
		panic(err)
	}
	client := paho.NewClient(opts)
	L.Info("Connecting to Mqtt broker.", "serveur", server)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	return client
}
