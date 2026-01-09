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
- **DLNA/UPnP transport** - Send images via AVTransport
- **Text rendering** - Built-in text-to-image for simple messages
- **Thread-safe** - Safe for concurrent use

## Supported TVs

Tested with:
- JVC Smart TVs (VIDAA platform)
- Other DLNA-compatible Smart TVs

## License

MIT
