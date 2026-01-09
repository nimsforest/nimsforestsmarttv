package nimsforestsmarttv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestDisplayHLS tests the HLS streaming functionality
func TestDisplayHLS(t *testing.T) {
	// Create a mock TV server that accepts SOAP requests
	var receivedAction string
	var receivedBody string

	mockTV := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAction = r.Header.Get("SOAPAction")

		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])

		// Return success response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:SetAVTransportURIResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
    </u:SetAVTransportURIResponse>
  </s:Body>
</s:Envelope>`))
	}))
	defer mockTV.Close()

	// Create a TV pointing to our mock server
	tv := &TV{
		Name:       "Test TV",
		IP:         "127.0.0.1",
		Port:       80,
		ControlURL: mockTV.URL,
		BaseURL:    mockTV.URL,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test setAVTransportURIForVideo directly
	hlsURL := "https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8"
	err := tv.setAVTransportURIForVideo(ctx, hlsURL, "Test Stream")
	if err != nil {
		t.Fatalf("setAVTransportURIForVideo failed: %v", err)
	}

	// Verify the SOAP action
	if !strings.Contains(receivedAction, "SetAVTransportURI") {
		t.Errorf("Expected SetAVTransportURI action, got: %s", receivedAction)
	}

	// Verify the body contains the HLS URL
	if !strings.Contains(receivedBody, hlsURL) {
		t.Errorf("Expected body to contain HLS URL, got: %s", receivedBody)
	}

	// Verify the content type is set correctly for HLS
	if !strings.Contains(receivedBody, "application/x-mpegURL") {
		t.Errorf("Expected body to contain application/x-mpegURL content type for .m3u8 URL")
	}

	// Verify the title is included
	if !strings.Contains(receivedBody, "Test Stream") {
		t.Errorf("Expected body to contain title 'Test Stream'")
	}
}

// TestDisplayHLSWithRenderer tests HLS through the Renderer
func TestDisplayHLSWithRenderer(t *testing.T) {
	// Track requests
	var requests []string

	mockTV := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.Header.Get("SOAPAction")
		requests = append(requests, action)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body></s:Body>
</s:Envelope>`))
	}))
	defer mockTV.Close()

	tv := &TV{
		Name:       "Test TV",
		IP:         "127.0.0.1",
		Port:       80,
		ControlURL: mockTV.URL,
		BaseURL:    mockTV.URL,
	}

	renderer, err := NewRenderer()
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}
	defer renderer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test DisplayHLS
	hlsURL := "https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8"
	err = renderer.DisplayHLS(ctx, tv, hlsURL, "My Test Stream")
	if err != nil {
		t.Fatalf("DisplayHLS failed: %v", err)
	}

	// Should have received SetAVTransportURI and Play
	if len(requests) != 2 {
		t.Errorf("Expected 2 requests (SetAVTransportURI + Play), got %d", len(requests))
	}

	if len(requests) >= 1 && !strings.Contains(requests[0], "SetAVTransportURI") {
		t.Errorf("First request should be SetAVTransportURI, got: %s", requests[0])
	}

	if len(requests) >= 2 && !strings.Contains(requests[1], "Play") {
		t.Errorf("Second request should be Play, got: %s", requests[1])
	}
}

// TestVideoContentTypes tests that different video URLs get correct content types
func TestVideoContentTypes(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		expectedType string
	}{
		{
			name:         "HLS m3u8 stream",
			url:          "https://example.com/stream.m3u8",
			expectedType: "application/x-mpegURL",
		},
		{
			name:         "HLS with query params",
			url:          "https://example.com/stream?format=m3u8&token=abc",
			expectedType: "application/x-mpegURL",
		},
		{
			name:         "MPEG-TS stream",
			url:          "https://example.com/stream.ts",
			expectedType: "video/mp2t",
		},
		{
			name:         "Generic video URL",
			url:          "https://example.com/video",
			expectedType: "video/mp2t",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedBody string

			mockTV := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				buf := make([]byte, 8192)
				n, _ := r.Body.Read(buf)
				receivedBody = string(buf[:n])

				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body></s:Body></s:Envelope>`))
			}))
			defer mockTV.Close()

			tv := &TV{
				Name:       "Test TV",
				ControlURL: mockTV.URL,
			}

			ctx := context.Background()
			err := tv.setAVTransportURIForVideo(ctx, tt.url, "Test")
			if err != nil {
				t.Fatalf("setAVTransportURIForVideo failed: %v", err)
			}

			if !strings.Contains(receivedBody, tt.expectedType) {
				t.Errorf("Expected content type %s in body, got: %s", tt.expectedType, receivedBody)
			}
		})
	}
}

// TestDisplayHLSDefaultTitle tests that empty title gets a default
func TestDisplayHLSDefaultTitle(t *testing.T) {
	var receivedBody string

	mockTV := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 8192)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body></s:Body></s:Envelope>`))
	}))
	defer mockTV.Close()

	tv := &TV{
		Name:       "Test TV",
		ControlURL: mockTV.URL,
	}

	renderer, err := NewRenderer()
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}
	defer renderer.Close()

	ctx := context.Background()
	err = renderer.DisplayHLS(ctx, tv, "https://example.com/stream.m3u8", "")
	if err != nil {
		t.Fatalf("DisplayHLS failed: %v", err)
	}

	// Should contain default title
	if !strings.Contains(receivedBody, "HLS Stream") {
		t.Errorf("Expected default title 'HLS Stream' in body when empty title provided")
	}
}

// ExampleDisplayHLS demonstrates how to stream HLS to a TV
func ExampleDisplayHLS() {
	ctx := context.Background()

	// Discover TVs
	tvs, err := Discover(ctx, 5*time.Second)
	if err != nil || len(tvs) == 0 {
		return
	}

	// Create renderer
	renderer, err := NewRenderer()
	if err != nil {
		return
	}
	defer renderer.Close()

	// Public test HLS stream (Big Buck Bunny)
	hlsURL := "https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8"

	// Stream to TV
	err = renderer.DisplayHLS(ctx, &tvs[0], hlsURL, "Big Buck Bunny")
	if err != nil {
		return
	}
}
