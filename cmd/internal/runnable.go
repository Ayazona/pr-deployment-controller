package internal

import (
	"context"
)

// Runnable implements the controller-manager runnable interface (used to detect lock acquisition)
type Runnable struct {
	ctx    context.Context
	cancel context.CancelFunc

	fanout       context.Context
	fanoutcancel context.CancelFunc
}

// NewRunnable creates a new runnable
func NewRunnable() *Runnable {
	ctx, cancel := context.WithCancel(context.Background())
	fanout, fanoutcancel := context.WithCancel(context.Background())

	return &Runnable{
		ctx:    ctx,
		cancel: cancel,

		fanout:       fanout,
		fanoutcancel: fanoutcancel,
	}
}

// Start implements the runnable interface
func (r *Runnable) Start(stop <-chan struct{}) error {
	// Cancel the wait context to kick off other processes
	r.fanoutcancel()

	// Sleep until Close is called
	<-r.ctx.Done()

	return nil
}

// Done returns a channel that closes when the Start function gets called
func (r *Runnable) Done() <-chan struct{} {
	return r.fanout.Done()
}

// Close releases the Wait call
func (r *Runnable) Close() {
	r.fanoutcancel()
	r.cancel()
}
