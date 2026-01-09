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
	// Encode image as JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
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
