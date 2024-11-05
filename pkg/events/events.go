package events

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/tom-ha/musiloo/pkg/playback"
	"github.com/zmb3/spotify/v2"
	"go.uber.org/zap"
)

type MessagePayload struct {
	MotionDetected bool `json:"motion_detected"`
}

type Config struct {
	Server   string
	Topic    string
	ClientID string
	Username string
	Password string
	QOS      int
}

func OnMessageReceived(ctx context.Context, spotifyClient *spotify.Client, spotifyURI string, message MQTT.Message, logger *zap.SugaredLogger) {
	var payload MessagePayload
	err := json.Unmarshal(message.Payload(), &payload)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to parse message payload: %v", err))
	}

	err = playback.HandlePlayback(ctx, spotifyClient, payload.MotionDetected, spotifyURI, logger)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to handle playback: %v", err))
	}
}

func GetMQTTConnOptions(config Config, callback MQTT.MessageHandler, logger *zap.SugaredLogger) (mqttOptions *MQTT.ClientOptions) {

	connOptions := MQTT.NewClientOptions().AddBroker(config.Server).SetClientID(config.ClientID).SetCleanSession(true)
	if config.Username != "" {
		connOptions.SetUsername(config.Username)
		if config.Password != "" {
			connOptions.SetPassword(config.Password)
		}
	}
	tlsConfig := &tls.Config{InsecureSkipVerify: true, ClientAuth: tls.NoClientCert}
	connOptions.SetTLSConfig(tlsConfig)

	connOptions.OnConnect = func(c MQTT.Client) {
		if token := c.Subscribe(config.Topic, byte(config.QOS), callback); token.Wait() && token.Error() != nil {
			logger.Fatal(fmt.Sprintln(token.Error()))
		}
	}

	return connOptions
}
