# nimsforestsmarttv

Go library for discovering and rendering to Smart TVs via DLNA/UPnP.

## Installation

```bash
go get github.com/nimsforest/nimsforestsmarttv
```

## Usage

### As a library

```go
package main

import (
    "context"
    "time"

    smarttv "github.com/nimsforest/nimsforestsmarttv"
)

func main() {
    ctx := context.Background()

    // Discover TVs on the network
    tvs, err := smarttv.Discover(ctx, 5*time.Second)
    if err != nil {
        panic(err)
    }

    if len(tvs) == 0 {
        println("No TVs found")
        return
    }

    // Create a renderer
    renderer, err := smarttv.NewRenderer()
    if err != nil {
        panic(err)
    }
    defer renderer.Close()

    // Display text on the first TV
    err = renderer.DisplayText(ctx, &tvs[0], "Hello World!")
    if err != nil {
        panic(err)
    }

    // Or stream HLS video
    hlsURL := "https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8"
    err = renderer.DisplayHLS(ctx, &tvs[0], hlsURL, "My Stream")
    if err != nil {
        panic(err)
    }
}
```

### Interactive CLI

```bash
go run ./cmd/smarttv
```

Commands:
- Type any text to display it on the TV
- `/discover` - Re-scan for TVs
- `/select` - Choose a different TV
- `/stop` - Stop displaying
- `/quit` - Exit

## Features

- **Zero external dependencies** - Standard library only
- **SSDP discovery** - Automatically find Smart TVs on your network
- **DLNA/UPnP transport** - Send images and video streams via AVTransport
- **HLS streaming** - Stream HLS video directly to your TV
- **Text rendering** - Built-in text-to-image for simple messages
- **Thread-safe** - Safe for concurrent use

## HLS Streaming

The library supports streaming HLS (HTTP Live Streaming) content to your TV. The TV fetches and plays the stream directly.

```go
// Stream HLS to TV
hlsURL := "https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8"
err := renderer.DisplayHLS(ctx, tv, hlsURL, "Big Buck Bunny")

// DisplayVideo is an alias that works the same way
err := renderer.DisplayVideo(ctx, tv, videoURL, "My Video")
```

Supported formats:
- HLS streams (.m3u8) - Uses `application/x-mpegURL` content type
- MPEG-TS streams - Uses `video/mp2t` content type
- Other video URLs - Falls back to `video/mp2t`

The title parameter is displayed in the TV's UI during playback.

## Supported TVs

Tested with:
- JVC Smart TVs (VIDAA platform)
- Other DLNA-compatible Smart TVs

## License

MIT
