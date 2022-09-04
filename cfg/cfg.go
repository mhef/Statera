// Package cfg handle the configurantions of the application.
package cfg

import (
	"bytes"
	"encoding/json"
	"io"
)

// Certificate hold the certificate and key files for use on TLS.
type Certificate struct {
	// CertFile hold the certificate file path location.
	CertFile string `json:"cert_file"`

	// CertFile hold the key file path location.
	KeyFile string `json:"key_file"`
}

// TLS define specific configurations for TLS.
type TLS struct {
	// Certs hold the certificates of the listener.
	Certs []Certificate `json:"certs"`

	// MinTLSVersion define the minimum TLS version supported by the listener.
	// If zero, TLS 1.0 is the default.
	MinTLSVersion uint16 `json:"min_tls_version"`

	// MaxTLSVersion define the maximum TLS version supported by the listener.
	// If zero, TLS 1.3 is the default.
	MaxTLSVersion uint16 `json:"max_tls_version"`
}

// Listener is, essentially, a opened port on the server that will wait for
// connections and requests.
type Listener struct {
	// Addr specifies the TCP address for the listener to listen on, in the form
	// "host:port".
	Addr string `json:"addr"`

	// HTTP2 define if the support for HTTP2 should be enabled for this listener.
	HTTP2 bool `json:"http2"`

	// TLS specifies the TLS configurations of the listener.
	//
	// If TLS.Certificates has at least one certificate, the listener will use HTTPS.
	TLS *TLS `json:"tls"`
}

type Rule struct {
	Priority   int    `json:"priority"`
	Listener   string `json:"listener"`
	Conditions []struct {
		Not       bool   `json:"not"`
		Type      int    `json:"type"`
		Key       string `json:"key"`
		Operation int    `json:"operation"`
		Value     string `json:"value"`
	} `json:"conditions"`
	Action struct {
		NodeGroup string `json:"node_group"`
		Reject    struct {
			StatusCode int    `json:"status_code"`
			Message    string `json:"message"`
		} `json:"reject"`
		Redirect string `json:"redirect"`
	} `json:"action"`
	Dynamic string `json:"dynamic"`
}

// NodeGroup is a group of target nodes servers.
type NodeGroup struct {
	// Name specifies the name of the group.
	Name string `json:"name"`

	// Nodes hold the address of the target nodes.
	Nodes []struct {
		Host string `json:"host"`
		Port uint16 `json:"port"`

		Weight int `json:"weight"`
	} `json:"nodes"`

	// HTTPS define if the connections to this group must use HTTPS.
	HTTPS bool `json:"https"`

	// Algorithm define the load balancing algorithm used to route requests to
	// this group.
	Algorithm string `json:"algorithm"`

	// HealthCheck define the health check configuration of the group.
	HealthCheck struct {
		// Path define the path to wich the health check requests should be
		// sent.
		Path string `json:"path"`

		// Interval define the interval in seconds between each health check
		// request.
		Interval int `json:"interval"`

		// Timeout define the time in seconds to a health check request be
		// considered failed.
		Timeout int `json:"timeout"`
	} `json:"health_check"`
}

// Config is a struct describing the complete configuration of the application.
type Config struct {
	Listeners  []Listener  `json:"listeners"`
	NodeGroups []NodeGroup `json:"node_groups"`
	Rules      []Rule      `json:"rules"`
}

// Load the configuration JSON from Reader and parse it.
func Load(r io.Reader) (*Config, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var ret Config
	if err := json.Unmarshal(b, &ret); err != nil {
		return nil, err
	}
	return &ret, nil
}

// Write the configuration JSON to Writer.
func (c *Config) Write(w io.Writer) error {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	if err := enc.Encode(c); err != nil {
		return err
	}
	if _, err := w.Write(buf.Bytes()); err != nil {
		return err
	}
	return nil
}
