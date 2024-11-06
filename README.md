# Musiloo

Musiloo is a small application that listens to MQTT topics and triggers a Spotify playlist.  

## Prerequisites

- Go 1.x or higher
- An MQTT broker (e.g., Mosquitto)
- A Spotify Premium account
- Spotify Developer Application credentials

## Configuration

The application is configured through environment variables:

### MQTT Configuration
- `MQTT_SERVER`: MQTT broker URL (default: `tcp://127.0.0.1:1883`)
- `MQTT_TOPIC`: Topic to subscribe to (default: `#`)
- `MQTT_CLIENT_ID`: Client identifier (default: system hostname)
- `MQTT_USERNAME`: MQTT username (optional)
- `MQTT_PASSWORD`: MQTT password (optional)
- `MQTT_QOS`: Quality of Service level (default: `2`)

### Spotify Configuration
- `SPOTIFY_PLAYLIST_ID`: ID of the Spotify playlist to play (required)
    - You can get this from the Spotify URL: `https://open.spotify.com/playlist/<PLAYLIST_ID>`


## Usage

The easiest way to run Musiloo is using Docker. The official image is available at:
```
ghcr.io/tom-ha/musiloo:latest
```

### Running with Docker

1. Create a `.env` file with your configuration:
    ```env
    SPOTIFY_PLAYLIST_ID=<playlist_id>
    MQTT_SERVER=<tcp://your.mqtt.broker:1883>
    MQTT_TOPIC=<mqtt_topic>
    MQTT_USERNAME=<mqtt_username>
    MQTT_PASSWORD=<mqtt_password>
    ```

2. Run the container:
    ```bash
    docker run -d \
      --name musiloo \
      --env-file .env \
      -p 8080:8080 \
      ghcr.io/tom-ha/musiloo:latest
    ```

### Running locally
1. First, set up your environment variables:
    ```bash
    export SPOTIFY_PLAYLIST_ID="your_playlist_id"
    export MQTT_SERVER="tcp://your.mqtt.broker:1883"
    export MQTT_TOPIC="home/music/#"
    # Add any other configuration as needed
    ```

2. Run the application:
    ```bash
    go run main.go
    ```