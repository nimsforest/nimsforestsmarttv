package nimsforestsmarttv

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	ssdpAddr    = "239.255.255.250:1900"
	ssdpSearch  = "M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\nST: urn:schemas-upnp-org:device:MediaRenderer:1\r\n\r\n"
)

// ssdpResponse represents a parsed SSDP response
type ssdpResponse struct {
	Location string
	Server   string
	USN      string
}

// Discover finds Smart TVs on the local network using SSDP.
// It returns a list of discovered TVs within the given timeout.
func Discover(ctx context.Context, timeout time.Duration) ([]TV, error) {
	// Create UDP connection for multicast
	addr, err := net.ResolveUDPAddr("udp4", ssdpAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve SSDP address: %w", err)
	}

	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return nil, fmt.Errorf("listen UDP: %w", err)
	}
	defer conn.Close()

	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(timeout))

	// Send M-SEARCH request
	_, err = conn.WriteToUDP([]byte(ssdpSearch), addr)
	if err != nil {
		return nil, fmt.Errorf("send SSDP search: %w", err)
	}

	// Collect responses
	seen := make(map[string]bool)
	var tvs []TV

	buf := make([]byte, 65535)
	for {
		select {
		case <-ctx.Done():
			return tvs, ctx.Err()
		default:
		}

		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			// Timeout or other error - we're done collecting
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			break
		}

		resp := parseSSDP(string(buf[:n]))
		if resp.Location == "" {
			continue
		}

		// Skip if we've already seen this device
		if seen[resp.Location] {
			continue
		}
		seen[resp.Location] = true

		// Fetch device description and create TV struct
		tv, err := fetchTVInfo(ctx, resp.Location)
		if err != nil {
			// Skip devices we can't get info for
			continue
		}

		tvs = append(tvs, *tv)
	}

	return tvs, nil
}

// parseSSDP parses an SSDP response into structured data
func parseSSDP(data string) ssdpResponse {
	var resp ssdpResponse

	reader := bufio.NewReader(strings.NewReader(data))

	// Read status line
	_, err := reader.ReadString('\n')
	if err != nil {
		return resp
	}

	// Parse headers
	for {
		line, err := reader.ReadString('\n')
		if err != nil || line == "\r\n" || line == "\n" {
			break
		}

		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch key {
		case "location":
			resp.Location = value
		case "server":
			resp.Server = value
		case "usn":
			resp.USN = value
		}
	}

	return resp
}

// fetchTVInfo fetches the device description XML and extracts TV information
func fetchTVInfo(ctx context.Context, location string) (*TV, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", location, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return parseDeviceDescription(resp.Body, location)
}
