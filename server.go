package nimsforestsmarttv

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// ImageServer serves images over HTTP for TVs to fetch
type ImageServer struct {
	server   *http.Server
	listener net.Listener
	localIP  string
	port     int

	mu      sync.RWMutex
	images  map[string][]byte
	counter uint64

	// Latest frame for streaming mode
	latestFrame     []byte
	latestFrameLock sync.RWMutex
}

// NewImageServer creates a new image server on an available port
func NewImageServer() (*ImageServer, error) {
	// Find local IP that can reach the network
	localIP, err := getLocalIP()
	if err != nil {
		return nil, fmt.Errorf("get local IP: %w", err)
	}

	// Listen on a random available port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port

	srv := &ImageServer{
		listener: listener,
		localIP:  localIP,
		port:     port,
		images:   make(map[string][]byte),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/stream.jpg", srv.handleStreamImage)
	mux.HandleFunc("/", srv.handleImage)

	srv.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start serving
	go srv.server.Serve(listener)

	return srv, nil
}

// handleStreamImage serves the latest frame (for streaming mode)
// Includes headers to encourage TV to re-fetch periodically
func (s *ImageServer) handleStreamImage(w http.ResponseWriter, r *http.Request) {
	s.latestFrameLock.RLock()
	data := s.latestFrame
	s.latestFrameLock.RUnlock()

	if data == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	// Aggressive no-cache to force re-fetch
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "Thu, 01 Jan 1970 00:00:00 GMT")
	// Refresh header - some clients honor this
	w.Header().Set("Refresh", "1")
	w.Write(data)
	fmt.Printf("[ImageServer] Stream: sent %d bytes to %s\n", len(data), r.RemoteAddr)
}

// handleImage serves stored images
func (s *ImageServer) handleImage(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("[ImageServer] Request: %s %s from %s\n", r.Method, r.URL.Path, r.RemoteAddr)

	s.mu.RLock()
	data, ok := s.images[r.URL.Path]
	s.mu.RUnlock()

	if !ok {
		fmt.Printf("[ImageServer] Not found: %s (have %d images)\n", r.URL.Path, len(s.images))
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	n, _ := w.Write(data)
	fmt.Printf("[ImageServer] Sent %d bytes\n", n)
}

// Store stores an image and returns its URL
func (s *ImageServer) Store(jpegData []byte) string {
	id := atomic.AddUint64(&s.counter, 1)
	path := fmt.Sprintf("/img_%d_%d.jpg", id, time.Now().UnixNano())

	s.mu.Lock()
	// Clean up old images (keep only last 10)
	if len(s.images) > 10 {
		for k := range s.images {
			delete(s.images, k)
			break
		}
	}
	s.images[path] = jpegData
	s.mu.Unlock()

	return fmt.Sprintf("http://%s:%d%s", s.localIP, s.port, path)
}

// UpdateLatestFrame updates the latest frame for streaming mode
func (s *ImageServer) UpdateLatestFrame(jpegData []byte) {
	s.latestFrameLock.Lock()
	s.latestFrame = jpegData
	s.latestFrameLock.Unlock()
}

// StreamURL returns the URL for the streaming endpoint
func (s *ImageServer) StreamURL() string {
	return fmt.Sprintf("http://%s:%d/stream.jpg", s.localIP, s.port)
}

// Close shuts down the image server
func (s *ImageServer) Close() error {
	return s.server.Close()
}

// URL returns the base URL of the image server
func (s *ImageServer) URL() string {
	return fmt.Sprintf("http://%s:%d", s.localIP, s.port)
}

// getLocalIP returns the local IP address that can reach external networks
func getLocalIP() (string, error) {
	// Connect to a known external address (doesn't actually send packets)
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		// Fallback: try to find any non-loopback IP
		return getLocalIPFallback()
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

// getLocalIPFallback finds any non-loopback IPv4 address
func getLocalIPFallback() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no suitable local IP found")
}
