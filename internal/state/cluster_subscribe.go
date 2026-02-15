package state

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/grepplabs/loggo/zlog"
)

const affiliateSubBuffer = 64

type AffiliateEventType int

const (
	AffiliateEventUpsert AffiliateEventType = iota
	AffiliateEventDelete
)

type AffiliateEvent struct {
	Type AffiliateEventType
	AffiliateInfo
}

type Subscription struct {
	id uint64

	ch    chan AffiliateEvent
	errCh chan error

	closed    atomic.Bool
	closeOnce sync.Once
	closeFn   func() // removes from cluster + closes chans
}

func (sub *Subscription) Events() <-chan AffiliateEvent { return sub.ch }
func (sub *Subscription) Errors() <-chan error          { return sub.errCh }

func (sub *Subscription) Close() {
	sub.closeOnce.Do(func() {
		sub.closed.Store(true)
		if sub.closeFn != nil {
			sub.closeFn()
		}
	})
}

func (sub *Subscription) notify(ev AffiliateEvent) {
	if sub.closed.Load() {
		return
	}

	select {
	case sub.ch <- ev:
		return
	default:
		// buffer full => watcher is too slow
	}

	// best-effort error delivery (don't block)
	select {
	case sub.errCh <- errors.New("lost update"):
	default:
	}

	sub.Close()
}

// SubscribeWithSnapshot returns a Subscription (events + errors) + cancel/close. Closing removes it from the cluster.
func (s *Cluster) SubscribeWithSnapshot(ctx context.Context, clientIP netip.Addr) ([]AffiliateInfo, *Subscription, error) {
	subID := atomic.AddUint64(&s.nextSubID, 1)

	sub := &Subscription{
		id:    subID,
		ch:    make(chan AffiliateEvent, affiliateSubBuffer),
		errCh: make(chan error, 1),
	}

	sub.closeFn = func() {
		s.subsMu.Lock()
		_, owned := s.subscribers[subID]
		if owned {
			delete(s.subscribers, subID)
		}
		s.subsMu.Unlock()

		if owned {
			close(sub.ch)
			close(sub.errCh)
			s.prolongExpiry()
		}
	}

	s.affiliatesMu.Lock()
	snapshot, err := s.listAffiliatesLocked(ctx)
	if err != nil {
		s.affiliatesMu.Unlock()
		return nil, nil, err
	}

	if clientIP.IsValid() {
		zlog.Infow("Watch cluster", "id", s.id, "subscription", subID, "clientIP", clientIP)
	} else {
		zlog.Infow("Watch cluster", "id", s.id, "subscription", subID)
	}

	s.subsMu.Lock()
	s.subscribers[subID] = sub
	s.subsMu.Unlock()
	s.affiliatesMu.Unlock()

	// auto-close on ctx done
	go func() {
		<-ctx.Done()
		sub.Close()
	}()

	return snapshot, sub, nil
}

// Cluster fanout: delegate “slow consumer policy” to Subscription.notify(...)
func (s *Cluster) notify(ev AffiliateEvent) {
	s.subsMu.Lock()
	// copy to avoid holding lock while notify() may Close() and re-enter
	subs := make([]*Subscription, 0, len(s.subscribers))
	for _, sub := range s.subscribers {
		subs = append(subs, sub)
	}
	s.subsMu.Unlock()

	for _, sub := range subs {
		sub.notify(ev)
	}
}
