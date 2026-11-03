package session

import "time"

type entry struct {
	session    *Session
	id         string
	generation uint64
	attached   bool
	detached   time.Time
}
