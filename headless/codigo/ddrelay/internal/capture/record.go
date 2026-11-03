package capture

import "time"

type Direction string

const (
	ClientToServer Direction = "ClientToServer"
	ServerToClient Direction = "ServerToClient"
)

type Record struct {
	Origin      string    `json:"origin,omitempty"`
	Time        time.Time `json:"time"`
	Stream      uint64    `json:"stream"`
	Direction   Direction `json:"direction"`
	Source      string    `json:"source"`
	Destination string    `json:"destination"`
	Lost        int       `json:"lost,omitempty"`
	Data        []byte    `json:"data,omitempty"`
}
