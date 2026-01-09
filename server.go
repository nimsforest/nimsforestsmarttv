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

// handleImage serves stored images
func (s *ImageServer) handleImage(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	data, ok := s.images[r.URL.Path]
	s.mu.RUnlock()

	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Write(data)
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
