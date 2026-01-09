package nimsforestsmarttv

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TV represents a discovered Smart TV
type TV struct {
	Name       string // Friendly name (e.g., "TV Salon")
	IP         string // IP address
	Port       int    // UPnP port
	ControlURL string // Full AVTransport control endpoint URL
	BaseURL    string // Base URL for the device
}

// deviceDescription represents the UPnP device description XML
type deviceDescription struct {
	XMLName     xml.Name `xml:"root"`
	URLBase     string   `xml:"URLBase"`
	Device      device   `xml:"device"`
}

type device struct {
	FriendlyName string    `xml:"friendlyName"`
	Manufacturer string    `xml:"manufacturer"`
	ModelName    string    `xml:"modelName"`
	ServiceList  []service `xml:"serviceList>service"`
}

type service struct {
	ServiceType string `xml:"serviceType"`
	ControlURL  string `xml:"controlURL"`
}

// parseDeviceDescription parses the UPnP device description XML
func parseDeviceDescription(r io.Reader, location string) (*TV, error) {
	var desc deviceDescription
	if err := xml.NewDecoder(r).Decode(&desc); err != nil {
		return nil, fmt.Errorf("parse device description: %w", err)
	}

	// Parse the location URL to get base URL components
	locURL, err := url.Parse(location)
	if err != nil {
		return nil, fmt.Errorf("parse location URL: %w", err)
	}

	// Determine base URL
	baseURL := desc.URLBase
	if baseURL == "" {
		baseURL = fmt.Sprintf("%s://%s", locURL.Scheme, locURL.Host)
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Find AVTransport service
	var controlURL string
	for _, svc := range desc.Device.ServiceList {
		if strings.Contains(svc.ServiceType, "AVTransport") {
			controlURL = svc.ControlURL
			break
		}
	}

	if controlURL == "" {
		return nil, fmt.Errorf("no AVTransport service found")
	}

	// Build full control URL
	if !strings.HasPrefix(controlURL, "http") {
		if !strings.HasPrefix(controlURL, "/") {
			controlURL = "/" + controlURL
		}
		controlURL = baseURL + controlURL
	}

	// Extract port from host
	port := 80
	if locURL.Port() != "" {
		fmt.Sscanf(locURL.Port(), "%d", &port)
	}

	return &TV{
		Name:       desc.Device.FriendlyName,
		IP:         locURL.Hostname(),
		Port:       port,
		ControlURL: controlURL,
		BaseURL:    baseURL,
	}, nil
}

// setAVTransportURI sends the SetAVTransportURI SOAP action to the TV
func (tv *TV) setAVTransportURI(ctx context.Context, uri string) error {
	// Build DIDL-Lite metadata for image
	metadata := fmt.Sprintf(`<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/"><item id="1" parentID="0" restricted="1"><dc:title>Image</dc:title><upnp:class>object.item.imageItem.photo</upnp:class><res protocolInfo="http-get:*:image/jpeg:*">%s</res></item></DIDL-Lite>`, uri)

	// Escape for XML
	metadata = escapeXML(metadata)

	soap := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:SetAVTransportURI xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <InstanceID>0</InstanceID>
      <CurrentURI>%s</CurrentURI>
      <CurrentURIMetaData>%s</CurrentURIMetaData>
    </u:SetAVTransportURI>
  </s:Body>
</s:Envelope>`, uri, metadata)

	return tv.sendSOAP(ctx, "SetAVTransportURI", soap)
}

// play sends the Play SOAP action to the TV
func (tv *TV) play(ctx context.Context) error {
	soap := `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:Play xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <InstanceID>0</InstanceID>
      <Speed>1</Speed>
    </u:Play>
  </s:Body>
</s:Envelope>`

	return tv.sendSOAP(ctx, "Play", soap)
}

// setAVTransportURIForVideo sends the SetAVTransportURI SOAP action for video/HLS streams
func (tv *TV) setAVTransportURIForVideo(ctx context.Context, uri string, title string) error {
	// Determine content type based on URL
	contentType := "video/mp2t"
	upnpClass := "object.item.videoItem"

	// Check if it's an HLS stream
	if strings.HasSuffix(uri, ".m3u8") || strings.Contains(uri, "m3u8") {
		contentType = "application/x-mpegURL"
	}

	// Build DIDL-Lite metadata for video
	metadata := fmt.Sprintf(`<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/"><item id="1" parentID="0" restricted="1"><dc:title>%s</dc:title><upnp:class>%s</upnp:class><res protocolInfo="http-get:*:%s:*">%s</res></item></DIDL-Lite>`, title, upnpClass, contentType, uri)

	// Escape for XML
	metadata = escapeXML(metadata)

	soap := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:SetAVTransportURI xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <InstanceID>0</InstanceID>
      <CurrentURI>%s</CurrentURI>
      <CurrentURIMetaData>%s</CurrentURIMetaData>
    </u:SetAVTransportURI>
  </s:Body>
</s:Envelope>`, uri, metadata)

	return tv.sendSOAP(ctx, "SetAVTransportURI", soap)
}

// stop sends the Stop SOAP action to the TV
func (tv *TV) stop(ctx context.Context) error {
	soap := `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:Stop xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <InstanceID>0</InstanceID>
    </u:Stop>
  </s:Body>
</s:Envelope>`

	return tv.sendSOAP(ctx, "Stop", soap)
}

// setNextAVTransportURI sets the next content to play (for gapless transitions)
func (tv *TV) setNextAVTransportURI(ctx context.Context, uri string) error {
	metadata := fmt.Sprintf(`<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/"><item id="1" parentID="0" restricted="1"><dc:title>Image</dc:title><upnp:class>object.item.imageItem.photo</upnp:class><res protocolInfo="http-get:*:image/jpeg:*">%s</res></item></DIDL-Lite>`, uri)
	metadata = escapeXML(metadata)

	soap := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:SetNextAVTransportURI xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <InstanceID>0</InstanceID>
      <NextURI>%s</NextURI>
      <NextURIMetaData>%s</NextURIMetaData>
    </u:SetNextAVTransportURI>
  </s:Body>
</s:Envelope>`, uri, metadata)

	return tv.sendSOAP(ctx, "SetNextAVTransportURI", soap)
}

// sendSOAP sends a SOAP request to the TV's AVTransport control endpoint
func (tv *TV) sendSOAP(ctx context.Context, action string, body string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", tv.ControlURL, bytes.NewBufferString(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", fmt.Sprintf(`"urn:schemas-upnp-org:service:AVTransport:1#%s"`, action))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send SOAP request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for error checking
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SOAP error: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	// Check for UPnP error in response
	if bytes.Contains(respBody, []byte("<UPnPError")) {
		return fmt.Errorf("UPnP error: %s", string(respBody))
	}

	return nil
}

// escapeXML escapes special characters for XML
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// String returns a string representation of the TV
func (tv *TV) String() string {
	return fmt.Sprintf("%s (%s:%d)", tv.Name, tv.IP, tv.Port)
}
