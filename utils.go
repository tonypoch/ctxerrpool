package ctxerrpool

import (
	"context"
)

// dead determines if the Pool is dead.
func dead(death <-chan struct{}) bool {
	select {
	case <-death:
		return true
	default:
		return false
	}
}

// expired determines if a work item is dead. If it is, the context's error is returned.
func expired(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// finished cancels the context and decrements the wait pool only once. It should be called when the worker is no
// longer working on this workItem.
func (item *workItem) finished() {
	item.mux.Lock()
	if !item.decremented {
		item.cancel()
		item.decremented = true
		item.wg.Done()
	}
	item.mux.Unlock()
}
