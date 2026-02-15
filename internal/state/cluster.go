package state

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grepplabs/loggo/zlog"
)

type Cluster struct {
	id string

	expiry atomic.Int64

	affiliatesMu sync.Mutex
	affiliates   map[string]*Affiliate

	subsMu      sync.Mutex
	nextSubID   uint64
	subscribers map[uint64]*Subscription
}

func NewCluster(clusterId string) *Cluster {
	c := &Cluster{
		id:          clusterId,
		affiliates:  make(map[string]*Affiliate),
		subscribers: make(map[uint64]*Subscription),
	}
	c.expiry.Store(time.Now().Add(ClusterTTL).UnixNano())
	return c
}

func (s *Cluster) prolongExpiry() {
	s.expiry.Store(time.Now().Add(ClusterTTL).UnixNano())
}

func (s *Cluster) IsExpired() bool {
	return time.Now().UnixNano() > s.expiry.Load()
}

func (s *Cluster) IsEmpty() bool {
	s.affiliatesMu.Lock()
	noAffiliates := len(s.affiliates) == 0
	s.affiliatesMu.Unlock()

	s.subsMu.Lock()
	noSubscribers := len(s.subscribers) == 0
	s.subsMu.Unlock()

	return noAffiliates && noSubscribers
}

func (s *Cluster) UpdateAffiliate(_ context.Context, req AffiliateUpdateRequest) error {
	s.affiliatesMu.Lock()
	ev, err := s.updateAffiliateLocked(req)
	s.affiliatesMu.Unlock()

	if err != nil {
		return err
	}
	if ev != nil {
		s.notify(*ev)
	}
	return nil
}

// must be called with affiliatesMu held
func (s *Cluster) updateAffiliateLocked(req AffiliateUpdateRequest) (*AffiliateEvent, error) {
	expiry := time.Now().Add(req.TTL)

	affiliate, exists := s.affiliates[req.ID]
	if !exists {
		if len(s.affiliates) >= AffiliatesMax {
			return nil, ErrTooManyAffiliates
		}

		zlog.Infow("Affiliate created", "id", s.id, "affiliateId", req.ID)
		affiliate = &Affiliate{id: req.ID}
		s.affiliates[req.ID] = affiliate
	}

	dataChanged := false
	if len(req.Data) > 0 {
		affiliate.expiry = expiry
		affiliate.data = req.Data
		dataChanged = true
	}

	endpointsChanged, err := affiliate.MergeEndpoints(req.Endpoints, expiry)
	if err != nil {
		return nil, err
	}

	if !dataChanged && !endpointsChanged {
		return nil, nil
	}

	ev := &AffiliateEvent{
		Type:          AffiliateEventUpsert,
		AffiliateInfo: affiliate.ToInfo(),
	}

	return ev, nil
}

func (s *Cluster) DeleteAffiliate(_ context.Context, id string) error {
	s.affiliatesMu.Lock()
	ev := s.deleteAffiliateLocked(id)
	s.affiliatesMu.Unlock()

	if ev != nil {
		s.prolongExpiry()
		s.notify(*ev)
	}
	return nil
}

// must be called with affiliatesMu held
func (s *Cluster) deleteAffiliateLocked(id string) *AffiliateEvent {
	if _, ok := s.affiliates[id]; !ok {
		return nil
	}
	zlog.Infow("Affiliate deleted", "id", s.id, "affiliateId", id)
	delete(s.affiliates, id)

	return &AffiliateEvent{
		Type: AffiliateEventDelete,
		AffiliateInfo: AffiliateInfo{
			ID: id, // tombstone
		},
	}
}

func (s *Cluster) ListAffiliates(ctx context.Context) ([]AffiliateInfo, error) {
	s.affiliatesMu.Lock()
	defer s.affiliatesMu.Unlock()

	return s.listAffiliatesLocked(ctx)
}

// must be called with affiliatesMu held
// nolint:unparam
func (s *Cluster) listAffiliatesLocked(_ context.Context) ([]AffiliateInfo, error) {
	out := make([]AffiliateInfo, 0, len(s.affiliates))
	for _, a := range s.affiliates {
		out = append(out, a.ToInfo())
	}
	return out, nil
}

func (s *Cluster) Prune() {
	events := s.pruneAffiliates()
	for _, ev := range events {
		s.notify(ev)
	}
}

func (s *Cluster) pruneAffiliates() []AffiliateEvent {
	now := time.Now()
	var events []AffiliateEvent

	s.affiliatesMu.Lock()
	defer s.affiliatesMu.Unlock()

	for id, aff := range s.affiliates {
		if !aff.expiry.After(now) {
			delete(s.affiliates, id)
			zlog.Infow("Affiliate expired", "cluster", s.id, "affiliate", id)
			events = append(events, AffiliateEvent{
				Type:          AffiliateEventDelete,
				AffiliateInfo: AffiliateInfo{ID: id},
			})
			continue
		}

		newEndpoints := make(Endpoints, 0, len(aff.endpoints))
		changed := false
		for _, ep := range aff.endpoints {
			if ep.expiry.After(now) {
				newEndpoints = append(newEndpoints, ep)
			} else {
				changed = true
			}
		}

		if changed {
			aff.endpoints = newEndpoints
			zlog.Infow("Affiliate endpoints expired", "cluster", s.id, "affiliate", id)
			events = append(events, AffiliateEvent{
				Type:          AffiliateEventUpsert,
				AffiliateInfo: aff.ToInfo(),
			})
		}
	}
	return events
}
