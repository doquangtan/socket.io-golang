package engineio

import (
	"net/http"
	"net/url"
	"time"
)

// Handshake represents the Socket.IO handshake data
type Handshake struct {
	// Headers of the initial request
	Headers http.Header `json:"headers"`

	// Query params of the initial request
	Query url.Values `json:"query"`

	// Auth is the authentication payload sent by the client
	// e.g: { "token": "abc123" }
	Auth HandshakeAuth `json:"auth"`

	// Time is the date of creation as string
	Time string `json:"time"`

	// Issued is the date of creation as unix timestamp (ms)
	Issued int64 `json:"issued"`

	// URL is the request URL string
	URL string `json:"url"`

	// Address is the IP of the client
	Address string `json:"address"`

	// Xdomain indicates whether the connection is cross-domain
	Xdomain bool `json:"xdomain"`

	// Secure indicates whether the connection is secure (HTTPS/WSS)
	Secure bool `json:"secure"`
}

type HandshakeAuth struct {
	Token string `json:"token"`
}

type ConnParameters struct {
	PingInterval time.Duration
	PingTimeout  time.Duration
	SID          string
	Upgrades     []string
	MaxPayload   int
}

type jsonParameters struct {
	SID          string   `json:"sid"`
	Upgrades     []string `json:"upgrades"`
	PingInterval int      `json:"pingInterval,omitempty"`
	PingTimeout  int      `json:"pingTimeout,omitempty"`
	MaxPayload   int      `json:"maxPayload,omitempty"`
}

// func ReadConnParameters(r io.Reader) (ConnParameters, error) {
// 	var param jsonParameters
// 	if err := json.NewDecoder(r).Decode(&param); err != nil {
// 		return ConnParameters{}, err
// 	}

// 	return ConnParameters{
// 		SID:          param.SID,
// 		Upgrades:     param.Upgrades,
// 		PingInterval: time.Duration(param.PingInterval) * time.Millisecond,
// 		PingTimeout:  time.Duration(param.PingTimeout) * time.Millisecond,
// 		MaxPayload:   param.MaxPayload,
// 	}, nil
// }

func (p ConnParameters) ToJson() jsonParameters {
	arg := jsonParameters{
		SID:          p.SID,
		Upgrades:     p.Upgrades,
		PingInterval: int(p.PingInterval / time.Millisecond),
		PingTimeout:  int(p.PingTimeout / time.Millisecond),
		MaxPayload:   int(p.MaxPayload),
	}
	return arg
}
