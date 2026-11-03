package metrics

import (
	"cmp"
	"slices"
	"sync"
	"time"
)

const (
	maxRewardTypes         = 4096
	maxInstances           = 64
	maxInstanceRewardTypes = 128
)

type Recorder struct {
	mu             sync.RWMutex
	state          Snapshot
	total          *combat
	instances      map[InstanceKey]*combat
	current        *combat
	generation     uint64
	knownInstances int
}

func New(startedAt time.Time) *Recorder {
	return &Recorder{
		state:     Snapshot{StartedAt: startedAt},
		total:     newCombat(maxRewardTypes),
		instances: make(map[InstanceKey]*combat),
	}
}

func (r *Recorder) Snapshot() Snapshot {
	r.mu.RLock()
	snapshot := r.state
	snapshot.Totals = r.total.snapshot()
	snapshot.Instances = make([]Instance, 0, len(r.instances))
	for key, instance := range r.instances {
		snapshot.Instances = append(snapshot.Instances, Instance{Key: key, Totals: instance.snapshot()})
	}
	r.mu.RUnlock()

	slices.SortFunc(snapshot.Instances, func(a, b Instance) int {
		return cmp.Or(
			cmp.Compare(a.Key.Grouping, b.Key.Grouping),
			cmp.Compare(a.Key.RoomType, b.Key.RoomType),
			cmp.Compare(a.Key.InstanceID, b.Key.InstanceID),
			cmp.Compare(a.Key.Difficulty, b.Key.Difficulty),
		)
	})

	return snapshot
}

func (r *Recorder) Observe(o Observation) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if o.Sequence <= r.state.ObservedSequence {
		return
	}

	r.state.ObservedSequence = o.Sequence
	r.state.ObservedAt = o.ReceivedAt
	if o.Server {
		r.state.ServerPackets++
	} else {
		r.state.ClientPackets++
	}
	if !o.Decoded {
		r.state.IncompletePackets++
		return
	}
	if !o.Server {
		return
	}

	if r.generation != o.State.Fight.Generation {
		if r.current != nil {
			r.current.state.Fights.reset()
			r.current.resetMission()
		}
		r.current = nil
		r.generation = o.State.Fight.Generation
	}

	if r.current == nil && (o.Name == "fight_created" || o.Name == "fight_started") {
		r.selectInstance(o)
	}

	r.total.observe(o)
	if r.current != nil {
		r.current.observe(o)
	}
}

func (r *Recorder) selectInstance(o Observation) {
	key := InstanceKey{Grouping: InstanceGroupingUnknown}
	if o.State.Room.MapID != nil && o.State.Room.Difficulty != nil && o.State.Fight.RoomType != nil {
		key = InstanceKey{
			Grouping:   InstanceGroupingKnown,
			RoomType:   *o.State.Fight.RoomType,
			InstanceID: *o.State.Room.MapID,
			Difficulty: *o.State.Room.Difficulty,
		}
	}

	current, exists := r.instances[key]
	if exists {
		r.current = current
		return
	}
	if key.Grouping == InstanceGroupingKnown && r.knownInstances >= maxInstances {
		key = InstanceKey{Grouping: InstanceGroupingOverflow}
		current, exists = r.instances[key]
	}
	if !exists {
		current = newCombat(maxInstanceRewardTypes)
		r.instances[key] = current
		if key.Grouping == InstanceGroupingKnown {
			r.knownInstances++
		}
	}

	r.current = current
}
