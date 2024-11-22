package playback

import (
	"context"
	"fmt"

	"github.com/zmb3/spotify/v2"
	"go.uber.org/zap"
)

func startPlayback(ctx context.Context, client *spotify.Client, spotifyURI string) error {
	deviceIDs, err := client.PlayerDevices(ctx)
	if len(deviceIDs) <= 0 {
		return fmt.Errorf("no devices found")
	}
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

func HandlePlayback(ctx context.Context, client *spotify.Client, playMusic bool, spotifyURI string, logger *zap.SugaredLogger) error {
	if playMusic {
		err := startPlayback(ctx, client, spotifyURI)
		if err != nil {
			return err
		}
		logger.Info("playback started")
	} else {
		err := stopPlayback(ctx, client)
		if err != nil {
			return err
		}
		logger.Info("playback paused")
	}
	return nil
}
