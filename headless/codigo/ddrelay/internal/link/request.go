package link

import "sync"

type request struct {
	link     *Link
	reply    chan result
	changing bool
	finished sync.Once
}

func newRequest(link *Link, changing bool) *request {
	if changing {
		link.scriptChanges.Add(1)
	}

	return &request{
		link:     link,
		reply:    make(chan result, 1),
		changing: changing,
	}
}

func (r *request) release() {
	r.finished.Do(func() {
		if r.changing {
			r.link.scriptChanges.Add(-1)
		}
	})
}

func (r *request) respond(answer result) {
	r.release()
	r.reply <- answer
}
