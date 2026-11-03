package metrics

import "time"

type Activity struct {
	Started           uint64
	Completed         uint64
	Won               uint64
	Lost              uint64
	UnknownOutcomes   uint64
	Abandoned         uint64
	CompletedDuration time.Duration
	ActiveSince       time.Time
	finished          bool
	outcomeKnown      bool
}

func (a *Activity) reset() {
	if !a.ActiveSince.IsZero() {
		a.Abandoned++
	}

	a.ActiveSince = time.Time{}
	a.finished = false
	a.outcomeKnown = false
}

func (a *Activity) start(now time.Time) {
	if !a.ActiveSince.IsZero() || a.finished {
		return
	}

	a.Started++
	a.ActiveSince = now
}

func (a *Activity) finish(now time.Time, winner *bool) bool {
	if !a.ActiveSince.IsZero() {
		a.Completed++
		a.UnknownOutcomes++
		a.CompletedDuration += now.Sub(a.ActiveSince)
		a.ActiveSince = time.Time{}
		a.finished = true
	}

	if !a.finished || a.outcomeKnown || winner == nil {
		return false
	}

	a.outcomeKnown = true
	a.UnknownOutcomes--
	if *winner {
		a.Won++
	} else {
		a.Lost++
	}

	return true
}
