package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/tom-ha/musiloo/pkg/events"
	"github.com/tom-ha/musiloo/pkg/playback"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"go.uber.org/zap"
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
			qos, err := strconv.Atoi(getEnv("MQTT_QOS", "2"))
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

	spotifyClientID := getEnv("SPOTIFY_CLIENT_ID", "")
	if spotifyClientID == "" {
		logger.Fatal("Please specify SPOTIFY_CLIENT_ID")
	}

	spotifyClientSecret := getEnv("SPOTIFY_CLIENT_SECRET", "")
	if spotifyClientSecret == "" {
		logger.Fatal("Please specify SPOTIFY_CLIENT_SECRET")
	}

	redirectURI := "http://localhost:8080/callback"
	spotifyAuthConfig := spotifyauth.New(
		spotifyauth.WithRedirectURL(redirectURI),
		spotifyauth.WithScopes(
			spotifyauth.ScopeUserReadPrivate,
			spotifyauth.ScopeUserModifyPlaybackState,
			spotifyauth.ScopeUserReadPlaybackState),
		spotifyauth.WithClientID(spotifyClientID),
		spotifyauth.WithClientSecret(spotifyClientSecret),
	)

	spotifyChn := playback.AuthenticateSpotify(spotifyAuthConfig, logger)
	spotifyClient := <-spotifyChn

	ctx := context.Background()
	err := playback.InitPlayback(ctx, spotifyClient, spotifyPlaylistID)
	if err != nil {
		logger.Fatal(err)
	}
	connOptions := events.GetMQTTConnOptions(*mqttConfig, func(_ mqtt.Client, message mqtt.Message) {
		events.OnMessageReceived(ctx, spotifyClient, message, logger)
	}, logger)
	mqttClient := mqtt.NewClient(connOptions)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		logger.Fatal(token.Error())
	} else {
		logger.Infof("Connected successfully to %s\n", mqttConfig.Server)
	}

	<-mqttChn
}
