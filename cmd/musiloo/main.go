package main

import (
	"context"
	"github.com/eclipse/paho.mqtt.golang"
	"github.com/tom-ha/musiloo/pkg/events"
	"github.com/tom-ha/musiloo/pkg/playback"
	"github.com/zmb3/spotify/v2/auth"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

func getEnv(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	zapLogger, _ := zap.NewProduction()
	defer zapLogger.Sync()
	logger := zapLogger.Sugar()

	mqttChn := make(chan os.Signal, 1)
	signal.Notify(mqttChn, os.Interrupt, syscall.SIGTERM)

	hostname, _ := os.Hostname()

	mqttConfig := &events.Config{
		Server:   getEnv("MQTT_SERVER", "tcp://127.0.0.1:1883"),
		Topic:    getEnv("MQTT_TOPIC", "#"),
		ClientID: getEnv("MQTT_CLIENT_ID", hostname),
		Username: getEnv("MQTT_USERNAME", ""),
		Password: getEnv("MQTT_PASSWORD", ""),
		QOS: func() int {
			qos, err := strconv.Atoi(getEnv("MQTT_QOS", "0"))
			if err != nil {
				logger.Fatal("Failed to parse MQTT QOS")
			}
			return qos
		}(),
	}

	spotifyPlaylistID := getEnv("SPOTIFY_PLAYLIST_ID", "")
	if spotifyPlaylistID == "" {
		logger.Fatal("Please specify SPOTIFY_PLAYLIST_ID")
	}

	redirectURI := "http://localhost:8080/callback"
	spotifyAuthConfig := spotifyauth.New(
		spotifyauth.WithRedirectURL(redirectURI),
		spotifyauth.WithScopes(
			spotifyauth.ScopeUserReadPrivate,
			spotifyauth.ScopeUserModifyPlaybackState,
			spotifyauth.ScopeUserReadPlaybackState))

	spotifyChn := playback.AuthenticateSpotify(spotifyAuthConfig, logger)
	spotifyClient := <-spotifyChn

	ctx := context.Background()
	connOptions := events.GetMQTTConnOptions(*mqttConfig, func(_ mqtt.Client, message mqtt.Message) {
		events.OnMessageReceived(ctx, spotifyClient, playback.GetSpotifyURI(spotifyPlaylistID), message, logger)
	}, logger)
	mqttClient := mqtt.NewClient(connOptions)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		logger.Fatal(token.Error())
	} else {
		logger.Infof("Connected successfully to %s\n", mqttConfig.Server)
	}

	<-mqttChn
}