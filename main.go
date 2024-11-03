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

func handlePlayback(playMusic bool, ctx context.Context, client *spotify.Client, spotifyURI string) error {
	if playMusic {
		println("Playing music")
		err := client.PlayOpt(ctx, &spotify.PlayOptions{
			URIs: spotify.URI(spotifyURI),
		}
		if err != nil {
			return err
		}

	} else {
		println("Stop playing music")
	}
	return nil
}

func onMessageReceived(message MQTT.Message) {
	var payload MessagePayload
	err := json.Unmarshal(message.Payload(), &payload)
	if err != nil {
		fmt.Printf("failed to parse message payload: %v", err)
	}

	err = handlePlayback(payload.MotionDetected)
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

func getMQTTConnOptions(config MQTTConfig) (mqttOptions *MQTT.ClientOptions) {
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
		if token := c.Subscribe(
			config.topic,
			byte(config.qos),
			func(_ MQTT.Client, message MQTT.Message) { onMessageReceived(message) }); token.Wait() && token.Error() != nil {
			log.Fatal(token.Error())
		}
	}

	return connOptions
}

func authenticateSpotify() {
	const redirectURI = "http://localhost:8080/callback"

	var (
		auth = spotifyauth.New(
			spotifyauth.WithRedirectURL(redirectURI),
			spotifyauth.WithScopes(spotifyauth.ScopeUserReadPrivate, spotifyauth.ScopeUserModifyPlaybackState))
		ch    = make(chan *spotify.Client)
		state = "musiloo"
	)

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		spotifyHandlerConfig := &SpotifyHandlerConfig{
			writer: w,
			req:    r,
			auth:   auth,
			state:  state,
			chn:    ch,
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

	url := auth.AuthURL(state)
	fmt.Println("Please log in to Spotify by visiting the following page in your browser:", url)

	client := <-ch

	user, err := client.CurrentUser(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("You are logged in as:", user.ID)
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

func main() {
	MQTT.DEBUG = log.New(os.Stdout, "", 0)
	MQTT.ERROR = log.New(os.Stdout, "", 0)
	chn := make(chan os.Signal, 1)
	signal.Notify(chn, os.Interrupt, syscall.SIGTERM)

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

	spotifyPlaylistName := getEnv("SPOTIFY_PLAYLIST_NAME", "")
	if spotifyPlaylistName == "" {
		log.Fatal("Please specify SPOTIFY_PLAYLIST_NAME")
	}

	connOptions := getMQTTConnOptions(*mqttConfig)

	mqttClient := MQTT.NewClient(connOptions)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	} else {
		fmt.Printf("Connected successfully to %s\n", mqttConfig.server)
	}

	authenticateSpotify()

	<-chn
}
