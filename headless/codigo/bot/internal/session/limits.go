package session

import "time"

type Limits struct {
	Sessions, PerTenant, PerRelay int
	Grace, Timeout                time.Duration
}
