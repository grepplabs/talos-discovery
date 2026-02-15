package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	farFuture = time.Now().Add(time.Hour)
	farPast   = time.Now().Add(-time.Hour)
)

func TestAffiliate_ToInfo_ClonesData(t *testing.T) {
	a := &Affiliate{
		id:   "node-1",
		data: []byte("original"),
	}
	info := a.ToInfo()
	info.Data[0] = 'X'
	assert.Equal(t, []byte("original"), a.data, "ToInfo should clone data, not alias it")
}

func TestAffiliate_ToInfo_ClonesEndpoints(t *testing.T) {
	a := &Affiliate{
		id: "node-1",
		endpoints: Endpoints{
			{data: []byte("10.0.0.1"), expiry: farFuture},
		},
	}
	info := a.ToInfo()
	info.Endpoints[0][0] = 'X'
	assert.Equal(t, []byte("10.0.0.1"), a.endpoints[0].data, "ToInfo should clone endpoint data")
}

func TestAffiliate_ToInfo_EmptyEndpoints(t *testing.T) {
	a := &Affiliate{id: "node-1", data: []byte("d")}
	info := a.ToInfo()
	assert.NotNil(t, info.Endpoints)
	assert.Empty(t, info.Endpoints)
}

func TestMergeEndpoints_AddsNewEndpoint(t *testing.T) {
	a := &Affiliate{id: "node-1"}

	changed, err := a.MergeEndpoints([][]byte{[]byte("10.0.0.1")}, farFuture)
	require.NoError(t, err)
	assert.True(t, changed, "adding a new endpoint should report changed=true")
	require.Len(t, a.endpoints, 1)
	assert.Equal(t, []byte("10.0.0.1"), a.endpoints[0].data)
	assert.Equal(t, farFuture, a.endpoints[0].expiry)
}

func TestMergeEndpoints_AddsMultipleNewEndpoints(t *testing.T) {
	a := &Affiliate{id: "node-1"}

	changed, err := a.MergeEndpoints([][]byte{[]byte("10.0.0.1"), []byte("10.0.0.2")}, farFuture)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Len(t, a.endpoints, 2)
}

func TestMergeEndpoints_ClonesEndpointData(t *testing.T) {
	a := &Affiliate{id: "node-1"}

	ep := []byte("10.0.0.1")
	_, err := a.MergeEndpoints([][]byte{ep}, farFuture)
	require.NoError(t, err)

	ep[0] = 'X'
	assert.Equal(t, []byte("10.0.0.1"), a.endpoints[0].data, "endpoint data should be cloned on insert")
}

func TestMergeEndpoints_ExistingEndpoint_NotChanged(t *testing.T) {
	a := &Affiliate{
		id:        "node-1",
		endpoints: Endpoints{{data: []byte("10.0.0.1"), expiry: farFuture}},
	}

	changed, err := a.MergeEndpoints([][]byte{[]byte("10.0.0.1")}, farFuture)
	require.NoError(t, err)
	assert.False(t, changed, "re-registering an existing endpoint should not report changed")
	assert.Len(t, a.endpoints, 1)
}

func TestMergeEndpoints_ExistingEndpoint_ExtendsExpiry(t *testing.T) {
	later := time.Now().Add(2 * time.Hour)
	a := &Affiliate{
		id:        "node-1",
		endpoints: Endpoints{{data: []byte("10.0.0.1"), expiry: farFuture}},
	}

	changed, err := a.MergeEndpoints([][]byte{[]byte("10.0.0.1")}, later)
	require.NoError(t, err)
	assert.False(t, changed, "expiry extension should not count as a change")
	assert.Equal(t, later, a.endpoints[0].expiry)
}

func TestMergeEndpoints_ExistingEndpoint_DoesNotShortenExpiry(t *testing.T) {
	a := &Affiliate{
		id:        "node-1",
		endpoints: Endpoints{{data: []byte("10.0.0.1"), expiry: farFuture}},
	}
	earlier := time.Now().Add(time.Minute)

	_, err := a.MergeEndpoints([][]byte{[]byte("10.0.0.1")}, earlier)
	require.NoError(t, err)
	assert.Equal(t, farFuture, a.endpoints[0].expiry, "expiry should never be shortened")
}

func TestMergeEndpoints_ExtendsAffiliateExpiry(t *testing.T) {
	a := &Affiliate{id: "node-1", expiry: farPast}

	_, err := a.MergeEndpoints([][]byte{[]byte("10.0.0.1")}, farFuture)
	require.NoError(t, err)
	assert.Equal(t, farFuture, a.expiry)
}

func TestMergeEndpoints_DoesNotShortenAffiliateExpiry(t *testing.T) {
	a := &Affiliate{id: "node-1", expiry: farFuture}
	earlier := time.Now().Add(time.Minute)

	_, err := a.MergeEndpoints([][]byte{[]byte("10.0.0.1")}, earlier)
	require.NoError(t, err)
	assert.Equal(t, farFuture, a.expiry, "affiliate expiry should never be shortened")
}

func TestMergeEndpoints_ReturnsErrorAtLimit(t *testing.T) {
	a := &Affiliate{id: "node-1"}

	// Fill to the limit.
	for i := 0; i < AffiliateEndpointsMax; i++ {
		ep := []byte{byte(i >> 8), byte(i), 0, 1}
		_, err := a.MergeEndpoints([][]byte{ep}, farFuture)
		require.NoError(t, err)
	}
	require.Len(t, a.endpoints, AffiliateEndpointsMax)

	// One more should fail.
	_, err := a.MergeEndpoints([][]byte{[]byte("overflow")}, farFuture)
	require.ErrorIs(t, err, ErrTooManyEndpoints)
	assert.Len(t, a.endpoints, AffiliateEndpointsMax, "endpoint count should not change on error")
}

func TestMergeEndpoints_ExistingEndpointAtLimit_NotError(t *testing.T) {
	ep := []byte("10.0.0.1")
	a := &Affiliate{
		id:        "node-1",
		endpoints: make(Endpoints, AffiliateEndpointsMax),
	}
	a.endpoints[0] = Endpoint{data: ep, expiry: farFuture}

	// Re-registering an existing endpoint at the limit should be fine.
	changed, err := a.MergeEndpoints([][]byte{ep}, farFuture)
	require.NoError(t, err)
	assert.False(t, changed)
}

func TestMergeEndpoints_EmptyInput_NoChange(t *testing.T) {
	a := &Affiliate{id: "node-1", expiry: farFuture}

	changed, err := a.MergeEndpoints(nil, farFuture)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Empty(t, a.endpoints)
}
