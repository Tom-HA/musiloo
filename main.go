package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/zmb3/spotify/v2"
	"github.com/zmb3/spotify/v2/auth"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

type MessagePayload struct {
	MotionDetected bool `json:"motion_detected"`
}

type MQTTConfig struct {
	server   string
	topic    string
	clientID string
	username string
	password string
	qos      int
}
type SpotifyHandlerConfig struct {
	writer http.ResponseWriter
	req    *http.Request
	auth   *spotifyauth.Authenticator
	state  string
	chn    chan<- *spotify.Client
}

func startPlayback(ctx context.Context, client *spotify.Client, spotifyURI string) error {
	deviceIDs, err := client.PlayerDevices(ctx)
	if err != nil {
		return fmt.Errorf("failed to get player devices: %w", err)
	}
	uri := spotify.URI(spotifyURI)
	err = client.PlayOpt(ctx, &spotify.PlayOptions{
		DeviceID:        &deviceIDs[0].ID,
		PlaybackContext: &uri,
	})
	if err != nil {
		return err
	}
	return nil
}

func stopPlayback(ctx context.Context, client *spotify.Client) error {
	err := client.Pause(ctx)
	if err != nil {
		return err
	}
	return nil
}

func handlePlayback(ctx context.Context, client *spotify.Client, playMusic bool, spotifyURI string) error {
	if playMusic {
		err := startPlayback(ctx, client, spotifyURI)
		if err != nil {
			return err
		}
		fmt.Println("playback started")
	} else {
		err := stopPlayback(ctx, client)
		if err != nil {
			return err
		}
		fmt.Println("playback paused")
	}
	return nil
}

func onMessageReceived(ctx context.Context, spotifyClient *spotify.Client, spotifyURI string, message MQTT.Message) {
	var payload MessagePayload
	err := json.Unmarshal(message.Payload(), &payload)
	if err != nil {
		fmt.Printf("failed to parse message payload: %v", err)
	}

	err = handlePlayback(ctx, spotifyClient, payload.MotionDetected, spotifyURI)
	if err != nil {
		fmt.Printf("failed to handle playback: %v", err)
	}
}

func getEnv(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getMQTTConnOptions(config MQTTConfig, callback MQTT.MessageHandler) (mqttOptions *MQTT.ClientOptions) {

	connOptions := MQTT.NewClientOptions().AddBroker(config.server).SetClientID(config.clientID).SetCleanSession(true)
	if config.username != "" {
		connOptions.SetUsername(config.username)
		if config.password != "" {
			connOptions.SetPassword(config.password)
		}
	}
	tlsConfig := &tls.Config{InsecureSkipVerify: true, ClientAuth: tls.NoClientCert}
	connOptions.SetTLSConfig(tlsConfig)

	connOptions.OnConnect = func(c MQTT.Client) {
		if token := c.Subscribe(config.topic, byte(config.qos), callback); token.Wait() && token.Error() != nil {
			log.Fatal(token.Error())
		}
	}

	return connOptions
}

func authenticateSpotify(spotifyAuth *spotifyauth.Authenticator) (chn chan *spotify.Client) {
	chn = make(chan *spotify.Client)
	state := "musiloo"

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		spotifyHandlerConfig := &SpotifyHandlerConfig{
			writer: w,
			req:    r,
			auth:   spotifyAuth,
			state:  state,
			chn:    chn,
		}
		spotifyClientHandler(*spotifyHandlerConfig)
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Received request at:", r.URL.String())
	})
	go func() {
		err := http.ListenAndServe(":8080", nil)
		if err != nil {
			log.Fatal(err)
		}
	}()

	url := spotifyAuth.AuthURL(state)
	fmt.Println("Please log in to Spotify by visiting the following page in your browser:", url)

	return chn
}

func spotifyClientHandler(cfg SpotifyHandlerConfig) {
	tok, err := cfg.auth.Token(cfg.req.Context(), cfg.state, cfg.req)
	if err != nil {
		http.Error(cfg.writer, "Couldn't get token", http.StatusForbidden)
		log.Fatalln(err)
	}
	if st := cfg.req.FormValue("state"); st != cfg.state {
		http.NotFound(cfg.writer, cfg.req)
		log.Fatalf("State mismatch: %s != %s\n", st, cfg.state)
	}

	spotifyClient := spotify.New(cfg.auth.Client(cfg.req.Context(), tok))
	fmt.Printf("Login Completed!\n%s\n", cfg.writer)

	cfg.chn <- spotifyClient
}

func getSpotifyURI(playlistID string) string {
	return fmt.Sprintf("spotify:playlist:%s", playlistID)
}

func main() {
	MQTT.DEBUG = log.New(os.Stdout, "", 0)
	MQTT.ERROR = log.New(os.Stdout, "", 0)
	mqttChn := make(chan os.Signal, 1)
	signal.Notify(mqttChn, os.Interrupt, syscall.SIGTERM)

	hostname, _ := os.Hostname()

	mqttConfig := &MQTTConfig{
		server:   getEnv("MQTT_SERVER", "tcp://127.0.0.1:1883"),
		topic:    getEnv("MQTT_TOPIC", "#"),
		clientID: getEnv("MQTT_CLIENT_ID", hostname),
		username: getEnv("MQTT_USERNAME", ""),
		password: getEnv("MQTT_PASSWORD", ""),
		qos: func() int {
			qos, err := strconv.Atoi(getEnv("MQTT_QOS", "0"))
			if err != nil {
				log.Fatal("Failed to parse MQTT QOS")
			}
			return qos
		}(),
	}

	spotifyPlaylistID := getEnv("SPOTIFY_PLAYLIST_ID", "")
	if spotifyPlaylistID == "" {
		log.Fatal("Please specify SPOTIFY_PLAYLIST_ID")
	}

	redirectURI := "http://localhost:8080/callback"
	spotifyAuthConfig := spotifyauth.New(
		spotifyauth.WithRedirectURL(redirectURI),
		spotifyauth.WithScopes(
			spotifyauth.ScopeUserReadPrivate,
			spotifyauth.ScopeUserModifyPlaybackState,
			spotifyauth.ScopeUserReadPlaybackState))

	spotifyChn := authenticateSpotify(spotifyAuthConfig)
	spotifyClient := <-spotifyChn

	ctx := context.Background()
	connOptions := getMQTTConnOptions(*mqttConfig, func(_ MQTT.Client, message MQTT.Message) {
		onMessageReceived(ctx, spotifyClient, getSpotifyURI(spotifyPlaylistID), message)
	})
	mqttClient := MQTT.NewClient(connOptions)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	} else {
		fmt.Printf("Connected successfully to %s\n", mqttConfig.server)
	}

	<-mqttChn
}
