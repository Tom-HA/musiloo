package playback

import (
	"fmt"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"go.uber.org/zap"
	"log"
	"net/http"
)

type SpotifyHandlerConfig struct {
	writer http.ResponseWriter
	req    *http.Request
	auth   *spotifyauth.Authenticator
	state  string
	chn    chan<- *spotify.Client
}

func spotifyClientHandler(cfg SpotifyHandlerConfig) error {
	tok, err := cfg.auth.Token(cfg.req.Context(), cfg.state, cfg.req)
	if err != nil {
		http.Error(cfg.writer, "Couldn't get token", http.StatusForbidden)
		return err
	}
	if st := cfg.req.FormValue("state"); st != cfg.state {
		http.NotFound(cfg.writer, cfg.req)
		return fmt.Errorf("State mismatch: %s != %s\n", st, cfg.state)
	}

	spotifyClient := spotify.New(cfg.auth.Client(cfg.req.Context(), tok))

	cfg.chn <- spotifyClient
	return nil
}

func GetSpotifyURI(playlistID string) string {
	return fmt.Sprintf("spotify:playlist:%s", playlistID)
}

func AuthenticateSpotify(spotifyAuth *spotifyauth.Authenticator, logger *zap.SugaredLogger) (chn chan *spotify.Client) {
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
		err := spotifyClientHandler(*spotifyHandlerConfig)
		if err != nil {
			logger.Error("error handling callback", zap.Error(err))
		}
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
	logger.Info(fmt.Sprintf("Please log in to Spotify by visiting the following page in your browser: %s\n", url))

	return chn
}
