package nimsforestsmarttv

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"sync"
)

// Renderer is the high-level API for displaying content on Smart TVs
type Renderer struct {
	server *ImageServer
	mu     sync.Mutex

	// Text rendering options
	textOpts TextOptions
}

// Option configures a Renderer
type Option func(*Renderer)

// WithTextOptions sets the default text rendering options
func WithTextOptions(opts TextOptions) Option {
	return func(r *Renderer) {
		r.textOpts = opts
	}
}

// NewRenderer creates a new Renderer with an embedded image server
func NewRenderer(opts ...Option) (*Renderer, error) {
	server, err := NewImageServer()
	if err != nil {
		return nil, fmt.Errorf("create image server: %w", err)
	}

	r := &Renderer{
		server: server,
		textOpts: TextOptions{
			FontSize:   100,
			Width:      1920,
			Height:     1080,
			Color:      White,
			Background: Black,
		},
	}

	for _, opt := range opts {
		opt(r)
	}

	return r, nil
}

// Display renders an image on the given TV
func (r *Renderer) Display(ctx context.Context, tv *TV, img image.Image) error {
	// Convert to RGB (some TVs don't handle RGBA JPEGs well)
	bounds := img.Bounds()
	rgb := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgb.Set(x, y, img.At(x, y))
		}
	}

	// Encode image as baseline JPEG (most compatible)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, rgb, &jpeg.Options{Quality: 85}); err != nil {
		return fmt.Errorf("encode JPEG: %w", err)
	}

	return r.DisplayJPEG(ctx, tv, buf.Bytes())
}

// DisplayJPEG sends raw JPEG data to the TV
func (r *Renderer) DisplayJPEG(ctx context.Context, tv *TV, jpegData []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Store image on our server
	imageURL := r.server.Store(jpegData)

	// Send to TV
	if err := tv.setAVTransportURI(ctx, imageURL); err != nil {
		return fmt.Errorf("set URI: %w", err)
	}

	if err := tv.play(ctx); err != nil {
		return fmt.Errorf("play: %w", err)
	}

	return nil
}

// DisplayText renders text as an image and displays it on the TV
func (r *Renderer) DisplayText(ctx context.Context, tv *TV, text string) error {
	return r.DisplayTextWithOptions(ctx, tv, text, r.textOpts)
}

// DisplayTextWithOptions renders text with custom options
func (r *Renderer) DisplayTextWithOptions(ctx context.Context, tv *TV, text string, opts TextOptions) error {
	img := RenderText(text, opts)
	return r.Display(ctx, tv, img)
}

// DisplayHLS sends an HLS stream URL to the TV for playback.
// The TV will fetch and play the HLS stream directly.
// The title parameter is used for the stream's display name in the TV's UI.
func (r *Renderer) DisplayHLS(ctx context.Context, tv *TV, hlsURL string, title string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Default title if not provided
	if title == "" {
		title = "HLS Stream"
	}

	// Send the HLS URL to the TV
	if err := tv.setAVTransportURIForVideo(ctx, hlsURL, title); err != nil {
		return fmt.Errorf("set HLS URI: %w", err)
	}

	// Start playback
	if err := tv.play(ctx); err != nil {
		return fmt.Errorf("play HLS: %w", err)
	}

	return nil
}

// DisplayVideo sends a video URL to the TV for playback.
// This is an alias for DisplayHLS and works with various video stream formats.
func (r *Renderer) DisplayVideo(ctx context.Context, tv *TV, videoURL string, title string) error {
	return r.DisplayHLS(ctx, tv, videoURL, title)
}

// Stop stops playback on the TV
func (r *Renderer) Stop(ctx context.Context, tv *TV) error {
	return tv.stop(ctx)
}

// Close shuts down the renderer and its image server
func (r *Renderer) Close() error {
	return r.server.Close()
}

// ServerURL returns the URL of the embedded image server
func (r *Renderer) ServerURL() string {
	return r.server.URL()
}
