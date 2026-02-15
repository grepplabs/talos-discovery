package state

import (
	"bytes"
	"errors"
	"time"
)

var (
	ErrTooManyAffiliates = errors.New("too many affiliates in the cluster")
	ErrTooManyEndpoints  = errors.New("too many endpoints in the affiliate")
)

type AffiliateUpdateRequest struct {
	ID        string
	Data      []byte
	Endpoints [][]byte
	TTL       time.Duration
}

type Affiliate struct {
	id        string
	expiry    time.Time
	data      []byte
	endpoints Endpoints
}

type AffiliateInfo struct {
	ID        string
	Data      []byte
	Endpoints [][]byte
}

type Endpoints []Endpoint

type Endpoint struct {
	expiry time.Time
	data   []byte
}

func (e Endpoints) ToInfo() [][]byte {
	out := make([][]byte, 0, len(e))
	for i := range e {
		out = append(out, cloneBytes(e[i].data))
	}
	return out
}

func (a *Affiliate) ToInfo() AffiliateInfo {
	return AffiliateInfo{
		ID:        a.id,
		Data:      cloneBytes(a.data),
		Endpoints: a.endpoints.ToInfo(),
	}
}

func (a *Affiliate) findEndpointIndex(data []byte) int {
	for i := range a.endpoints {
		if bytes.Equal(a.endpoints[i].data, data) {
			return i
		}
	}
	return -1
}

// MergeEndpoints merges endpoints into the affiliate.
// It returns true only if the endpoint *set* changed (a new endpoint was added).
// Expiry extensions do NOT count as a change.
func (a *Affiliate) MergeEndpoints(endpoints [][]byte, expiry time.Time) (bool, error) {
	endpointsChanged := false

	for _, ep := range endpoints {
		idx := a.findEndpointIndex(ep)
		if idx >= 0 {
			// Extend TTL if needed (not considered a change)
			if a.endpoints[idx].expiry.Before(expiry) {
				a.endpoints[idx].expiry = expiry
			}
			continue
		}

		// add new endpoint
		if len(a.endpoints) >= AffiliateEndpointsMax {
			return false, ErrTooManyEndpoints
		}

		a.endpoints = append(a.endpoints, Endpoint{
			data:   cloneBytes(ep), // avoid aliasing
			expiry: expiry,
		})
		endpointsChanged = true
	}

	// extend affiliate expiry if needed (not considered a change)
	if a.expiry.Before(expiry) {
		a.expiry = expiry
	}

	return endpointsChanged, nil
}

func cloneBytes(b []byte) []byte {
	return append([]byte(nil), b...)
}
