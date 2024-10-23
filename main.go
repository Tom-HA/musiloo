package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type MessagePayload struct {
	MotionDetected bool `json:"motion_detected"`
}

func onMessageReceived(client MQTT.Client, message MQTT.Message) {
	var payload MessagePayload
	err := json.Unmarshal(message.Payload(), &payload)
	if err != nil {
		fmt.Errorf("failed to parse message payload: %w", err)
	}

	if payload.MotionDetected {
		fmt.Println("Motion detected!")
	}
}

func getEnv(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	MQTT.DEBUG = log.New(os.Stdout, "", 0)
	MQTT.ERROR = log.New(os.Stdout, "", 0)
	chn := make(chan os.Signal, 1)
	signal.Notify(chn, os.Interrupt, syscall.SIGTERM)

	hostname, _ := os.Hostname()

	server := getEnv("MQTT_SERVER", "tcp://127.0.0.1:1883")
	topic := getEnv("MQTT_TOPIC", "#")
	clientID := getEnv("MQTT_CLIENT_ID", hostname)
	username := getEnv("MQTT_USERNAME", "")
	password := getEnv("MQTT_PASSWORD", "")
	qos, err := strconv.Atoi(getEnv("MQTT_QOS", "0"))
	if err != nil {
		panic("Failed to parse MQTT QOS")
	}

	connOptions := MQTT.NewClientOptions().AddBroker(server).SetClientID(clientID).SetCleanSession(true)
	if username != "" {
		connOptions.SetUsername(username)
		if password != "" {
			connOptions.SetPassword(password)
		}
	}
	tlsConfig := &tls.Config{InsecureSkipVerify: true, ClientAuth: tls.NoClientCert}
	connOptions.SetTLSConfig(tlsConfig)

	connOptions.OnConnect = func(c MQTT.Client) {
		if token := c.Subscribe(topic, byte(qos), onMessageReceived); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	}

	client := MQTT.NewClient(connOptions)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	} else {
		fmt.Printf("Connected successfully to %s\n", server)
	}

	<-chn
}
