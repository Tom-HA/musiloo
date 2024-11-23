package playback

import (
	"context"
	"fmt"
	"time"

	"github.com/zmb3/spotify/v2"
	"go.uber.org/zap"
)

func getPlayOptions(ctx context.Context, client *spotify.Client, spotifyURI string) (playOptions *spotify.PlayOptions, err error) {
	deviceIDs, err := client.PlayerDevices(ctx)
	if len(deviceIDs) <= 0 {
		return nil, fmt.Errorf("no devices found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get player devices: %w", err)
	}
	uri := spotify.URI(spotifyURI)
	playOptions = &spotify.PlayOptions{
		DeviceID:        &deviceIDs[0].ID,
		PlaybackContext: &uri,
	}
	return playOptions, nil
}

func InitPlayback(ctx context.Context, client *spotify.Client, playlistID string) error {
	uri := GetSpotifyURI(playlistID)
	playOptions, err := getPlayOptions(ctx, client, uri)
	if err != nil {
		return err
	}
	err = client.VolumeOpt(ctx, 0, playOptions)
	if err != nil {
		return err
	}
	err = client.PlayOpt(ctx, playOptions)
	if err != nil {
		return err
	}
	err = client.PauseOpt(ctx, playOptions)
	if err != nil {
		return err
	}
	err = client.VolumeOpt(ctx, 100, playOptions)
	if err != nil {
		return err
	}

	return nil
}

func startPlayback(ctx context.Context, client *spotify.Client) error {
	err := client.Play(ctx)
	if err != nil {
		return err
	}
	maxVolume := 75
	for i := 20; i < maxVolume; i++ {
		err := client.Volume(ctx, i)
		if err != nil {
			return err
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}

func pausePlayback(ctx context.Context, client *spotify.Client) error {
	minVolume := 0
	for i := 75; i > minVolume; i-- {
		err := client.Volume(ctx, i)
		if err != nil {
			return err
		}
		time.Sleep(20 * time.Millisecond)
	}

	err := client.Pause(ctx)
	if err != nil {
		return err
	}
	return nil
}

func HandlePlayback(ctx context.Context, client *spotify.Client, playMusic bool, logger *zap.SugaredLogger) error {
	if playMusic {
		err := startPlayback(ctx, client)
		if err != nil {
			return err
		}
		logger.Info("playback started")
	} else {
		err := pausePlayback(ctx, client)
		if err != nil {
			return err
		}
		logger.Info("playback paused")
	}
	return nil
}
