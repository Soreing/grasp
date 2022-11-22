package grasp

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Poolable interface {
	PoolRelease() // Called when the resource is permanently removed from the pool
}

type resource struct {
	value  Poolable  // The pooled value
	expiry time.Time // Time till the resource is permanently removed
}

type request struct {
	ch  chan *resource  // Channel for serving an instance of the reqested resource
	ctx context.Context // Context of the request to allow for cancellation
}

type Pool struct {
	count  int             // Current count of resources
	limit  int             // Upper count limit of resources
	ttl    time.Duration   // Time To Live duration of idle resources
	constr func() Poolable // Constructs a new resource

	expiry bool        // Bool for whether an expiry is in progress
	timer  *time.Timer // Timer till the oldest resource is deleted

	exch  chan *resource // Resources arrive here when done
	reqs  chan *request  // Requests arrive here for resources
	close chan error     // Closes the pool and rejects requests

	icnt int         // Number of idle resources
	idle []*resource // Idle resources available for requests
	pend []*request  // Pending requests waiting for resources

	wg *sync.WaitGroup // Wait group for the pool to clean up
}

func NewPool(
	limit int,
	ttl time.Duration,
	constr func() Poolable,
) *Pool {
	p := &Pool{
		count:  0,
		limit:  limit,
		ttl:    ttl,
		constr: constr,

		expiry: false,
		timer:  time.NewTimer(time.Duration(0)),

		exch:  make(chan *resource, limit),
		reqs:  make(chan *request),
		close: make(chan error),

		icnt: 0,
		idle: make([]*resource, limit),
		pend: []*request{},

		wg: &sync.WaitGroup{},
	}

	<-p.timer.C
	p.wg.Add(1)
	go p.handler()
	return p
}

// Core handler of the pool, serving requests for resources and storing resources
// which have been freed up. Removes old resources if they have expired
func (p *Pool) handler() {
	defer p.wg.Done()
	for active := true; active; {
		if p.expiry {
			select {
			case req := <-p.reqs:
				p.serve(req)
			case res := <-p.exch:
				p.store(res)
			case <-p.timer.C:
				p.dropLast()
			case _, active = <-p.close:
				p.cleanup()
			}
		} else {
			select {
			case req := <-p.reqs:
				p.serve(req)
			case res := <-p.exch:
				p.store(res)
			case _, active = <-p.close:
				p.cleanup()
			}
		}
	}
}

// Serves a reqest or sets it as pending. If there is an idle resource, the request
// is immediately fulfilled. If there are no idle resources, one is created if there
// is capacity, otherwise the request will need to wait, and is set to pending.
func (p *Pool) serve(r *request) {
	if p.icnt == 0 {
		if p.count < p.limit {
			p.count++
			r.ch <- &resource{value: p.constr()}
		} else {
			p.pend = append(p.pend, r)
		}
	} else {
		p.icnt--
		r.ch <- p.idle[p.icnt]
		if p.icnt == 0 {
			p.timer.Stop()
			p.expiry = false
		}
	}
}

// Stores a resource that was freed up. If there is a pending request, the resource
// is immediately used to fulfill it. Otherwise, the respurce is set to be idle, and
// if idle for too long, gets removed. Cancelled requests are skipped
func (p *Pool) store(r *resource) {
	for idx := 0; idx < len(p.pend); {
		select {
		case <-p.pend[idx].ctx.Done():
			idx++
		default:
			p.pend[idx].ch <- r
			p.pend = p.pend[idx+1:]
			return
		}
	}

	r.expiry = time.Now().Add(p.ttl)
	p.idle[p.icnt] = r
	p.icnt++

	if p.icnt == 1 {
		p.expiry = true
		p.timer.Reset(time.Until(r.expiry))
	}
}

// Removes the oldest resource in the idle list permanently from the pool. The timer
// is set till the expiry of the next idle item, or canceled if the list is empty
func (p *Pool) dropLast() {
	p.idle[0].value.PoolRelease()
	p.count--
	p.icnt--
	
	for i := 0; i < p.icnt; i++ {
		p.idle[i] = p.idle[i+1] 
	}

	if p.icnt == 0 {
		p.expiry = false
	} else {
		p.timer.Reset(time.Until(p.idle[0].expiry))
	}
}

// Rejects pending requests, releases idle resources and waits for in use resources
// to return before quitting
func (p *Pool) cleanup() {
	for _, e := range p.pend {
		close(e.ch)
	}
	for i := p.icnt - 1; i >= 0; i-- {
		p.idle[i].value.PoolRelease()
		p.icnt--
		p.count--
	}
	for p.count > 0 {
		res := <-p.exch
		res.value.PoolRelease()
		p.count--
	}
}

// Makes a request to the pool to acquire a resource. If the pool is closed, returns
// with an error. If the resource is acquired, it is returned with a done function
// to be called when the resource is no longer needed
func (p *Pool) Acquire(ctx context.Context) (any, func(), error) {
	ch := make(chan *resource)
	select {
	case _, _ = <-p.close:
		return nil, nil, errors.New("pool's closed")
	default:
		p.reqs <- &request{
			ch:  ch,
			ctx: ctx,
		}

		select {
		case res, active := <-ch:
			done := func() {
				p.exch <- res
			}

			if active {
				return res.value, done, nil
			} else {
				return nil, nil, errors.New("pool's closed")
			}
		case <-ctx.Done():
			return nil, nil, errors.New("context canceled")
		}
	}

}

// Closes the pool
func (p *Pool) Close() {
	close(p.close)
	p.wg.Wait()
}

