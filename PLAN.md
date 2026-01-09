# Plan: nimsforestsmarttv Go Library

## Overview
Create a Go library (`nimsforest/nimsforestsmarttv`) that discovers Smart TVs on the network and sends images to them via DLNA/UPnP. This is a content-agnostic transport layer - it doesn't know about ViewModels or message content.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Caller (e.g., nimsforest2/smarttvview)                │
│  - Generates images (from ViewModel, messages, etc.)   │
└─────────────────────┬───────────────────────────────────┘
                      │ image bytes
                      ▼
┌─────────────────────────────────────────────────────────┐
│  smarttv-renderer library                              │
│  ┌─────────────┐  ┌─────────────┐  ┌────────────────┐  │
│  │  Discovery  │  │  Transport  │  │  Image Server  │  │
│  │  (SSDP)     │  │  (UPnP AVT) │  │  (HTTP)        │  │
│  └─────────────┘  └─────────────┘  └────────────────┘  │
└─────────────────────────────────────────────────────────┘
                      │
                      ▼
              ┌───────────────┐
              │   Smart TV    │
              │   (DLNA)      │
              └───────────────┘
```

## Package Structure

Go library with optional interactive CLI. Primary use is as library in nimsforest2, but CLI is available for testing and standalone use.

```
nimsforestsmarttv/
├── discovery.go      # SSDP discovery of TVs
├── tv.go             # TV struct and UPnP transport
├── server.go         # Built-in HTTP server for images
├── renderer.go       # High-level API (Renderer struct)
├── text.go           # Text-to-image rendering (for CLI messages)
├── example_test.go   # Usage examples as tests
├── cmd/
│   └── smarttv/
│       └── main.go   # Interactive CLI tool
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

## Core API Design

```go
package nimsforestsmarttv

// TV represents a discovered Smart TV
type TV struct {
    Name       string   // Friendly name (e.g., "TV Salon")
    IP         string   // IP address
    Port       int      // UPnP port
    ControlURL string   // AVTransport control endpoint
}

// Renderer is the high-level API
type Renderer struct {
    server *ImageServer  // Built-in HTTP server
    // ...
}

// Key methods:
func Discover(ctx context.Context, timeout time.Duration) ([]TV, error)
func NewRenderer(opts ...Option) (*Renderer, error)
func (r *Renderer) Display(ctx context.Context, tv *TV, img image.Image) error
func (r *Renderer) DisplayJPEG(ctx context.Context, tv *TV, jpegData []byte) error
func (r *Renderer) DisplayText(ctx context.Context, tv *TV, text string) error  // convenience
func (r *Renderer) Stop(ctx context.Context, tv *TV) error
func (r *Renderer) Close() error

// Text rendering options (for DisplayText and CLI)
type TextOptions struct {
    FontSize   int
    Color      color.Color
    Background color.Color
    Width      int  // default 1920
    Height     int  // default 1080
}
```

## Implementation Steps

### 1. Initialize repository
- Create repo `nimsforest/nimsforestsmarttv` via `gh repo create`
- **First commit**: Copy this plan as `PLAN.md` in repo root (so work can continue if interrupted)
- Initialize Go module: `github.com/nimsforest/nimsforestsmarttv`
- Add LICENSE (MIT) and README

### 2. SSDP Discovery (`discovery.go`)
- Send M-SEARCH to 239.255.255.250:1900
- Parse responses for MediaRenderer devices
- Fetch device description XML to get friendly name and control URLs
- Filter for DLNA-compatible TVs (VIDAA, etc.)

### 3. TV struct and UPnP transport (`tv.go`)
- Parse device description XML
- Implement SetAVTransportURI SOAP call
- Implement Play/Stop SOAP calls
- Handle DIDL-Lite metadata for images

### 4. Image Server (`server.go`)
- Embedded HTTP server to serve images
- Auto-detect local IP for TV to fetch from
- Unique URLs per image to bypass TV caching
- Cleanup old images

### 5. Renderer high-level API (`renderer.go`)
- Combines server + transport
- Simple `Display(tv, image)` that:
  1. Writes image to server
  2. Sends SetAVTransportURI + Play to TV
- Thread-safe for concurrent use

### 6. Text-to-image rendering (`text.go`)
- Simple text-to-image using Go's image/draw
- Configurable font size, colors, background
- Used by CLI for message display

### 7. Interactive CLI (`cmd/smarttv/main.go`)
- `go run ./cmd/smarttv` starts interactive mode
- On start: discovers TVs, shows selection menu
- Interactive prompt: type messages to display on TV
- Commands: `/discover`, `/select`, `/stop`, `/quit`
- Simple readline-style interface (no external deps)

### 8. Example tests (`example_test.go`)
- Runnable examples demonstrating discovery and display
- Serves as documentation and verification

## Dependencies
- Standard library only (net, net/http, encoding/xml, image, image/draw)
- No external dependencies for minimal footprint
- CLI uses bufio.Scanner for input (no readline library)

## Verification
1. Run `go test -v` in the package - examples demonstrate functionality
2. Run `go run ./cmd/smarttv` - interactive CLI should:
   - Discover TVs (find JVC at 192.168.68.61)
   - Allow selection if multiple TVs
   - Accept text input and display on TV
3. Import into nimsforest2 and test discovery + display

## Integration Guide for nimsforest2

After this library is complete, add to nimsforest2:

### 1. Add dependency
```bash
go get github.com/nimsforest/nimsforestsmarttv
```

### 2. Create `internal/smarttvview/smarttvview.go`
```go
package smarttvview

import (
    "context"
    "image"
    "sync"

    smarttv "github.com/nimsforest/nimsforestsmarttv"
    "github.com/nimsforest/nimsforest2/internal/viewmodel"
    "github.com/nimsforest/nimsforest2/internal/windwaker"
)

type SmartTVView struct {
    vm            *viewmodel.ViewModel
    renderer      *smarttv.Renderer
    tv            *smarttv.TV
    printInterval uint64
    beatCount     uint64
    mu            sync.Mutex
}

func New(vm *viewmodel.ViewModel, tv *smarttv.TV) (*SmartTVView, error) {
    renderer, err := smarttv.NewRenderer()
    if err != nil {
        return nil, err
    }
    return &SmartTVView{
        vm:            vm,
        renderer:      renderer,
        tv:            tv,
        printInterval: 90, // Update every ~1 second at 90Hz
    }, nil
}

// Dance implements windwaker.Dancer
func (v *SmartTVView) Dance(beat windwaker.Beat) error {
    v.mu.Lock()
    defer v.mu.Unlock()

    v.beatCount++
    if v.beatCount%v.printInterval != 0 {
        return nil
    }

    // Refresh ViewModel and render to image
    v.vm.Refresh()
    world := v.vm.GetWorld()
    img := v.renderWorldToImage(world)  // Implement this

    return v.renderer.Display(context.Background(), v.tv, img)
}

func (v *SmartTVView) ID() string {
    return "smarttv-view"
}

func (v *SmartTVView) renderWorldToImage(w *viewmodel.World) image.Image {
    // Convert ViewModel to image
    // Could reuse webview's grid positioning logic
    // Or create a simpler text-based summary image
}
```

### 3. Register in `cmd/forest/viewmodel.go`
```go
import smarttv "github.com/nimsforest/nimsforestsmarttv"

// Discover TVs
tvs, _ := smarttv.Discover(ctx, 5*time.Second)
if len(tvs) > 0 {
    tvView, _ := smarttvview.New(vm, &tvs[0])
    wind.Catch("dance.beat", tvView)
}
```

### 4. Alternative: Message Stream
For displaying messages (not ViewModel), use the library directly:
```go
import smarttv "github.com/nimsforest/nimsforestsmarttv"

renderer, _ := smarttv.NewRenderer()
tvs, _ := smarttv.Discover(ctx, 5*time.Second)

// Subscribe to a message stream and display
wind.Catch("messages.display", func(msg Message) {
    img := renderTextToImage(msg.Text)  // Your text-to-image function
    renderer.Display(ctx, &tvs[0], img)
})
```
