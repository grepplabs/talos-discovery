package state

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewState_WithSnapshot_LoadsState(t *testing.T) {
	path := snapshotPath(t)
	s := stateWith(t, "c1", "node-1", time.Hour, []byte("data"))
	require.NoError(t, s.SaveSnapshot(path))

	s2, err := NewState(WithSnapshot(path))
	require.NoError(t, err)

	affiliates, err := s2.ClusterFor("c1").ListAffiliates(context.Background())
	require.NoError(t, err)
	require.Len(t, affiliates, 1)
	assert.Equal(t, []byte("data"), affiliates[0].Data)
}

func TestNewState_WithSnapshot_FileNotExist_NoError(t *testing.T) {
	_, err := NewState(WithSnapshot("/tmp/does-not-exist-talos-discovery.bin"))
	assert.NoError(t, err, "missing snapshot file should be a no-op")
}

func TestNewState_WithSnapshot_CorruptFile_ReturnsError(t *testing.T) {
	path := snapshotPath(t)
	require.NoError(t, os.WriteFile(path, []byte("not valid protobuf"), 0o600))

	_, err := NewState(WithSnapshot(path))
	assert.Error(t, err)
}

func TestNewState_WithoutSnapshot_EmptyState(t *testing.T) {
	s, err := NewState()
	require.NoError(t, err)
	assert.NotNil(t, s)
}

func TestSaveSnapshot_CreatesFile(t *testing.T) {
	path := snapshotPath(t)
	s := stateWith(t, "c1", "node-1", time.Hour, []byte("data"))

	require.NoError(t, s.SaveSnapshot(path))

	_, err := os.Stat(path)
	assert.NoError(t, err, "snapshot file should exist after save")
}

func TestSaveSnapshot_EmptyState_CreatesFile(t *testing.T) {
	path := snapshotPath(t)
	s := newTestState(t)
	require.NoError(t, s.SaveSnapshot(path))

	_, err := os.Stat(path)
	assert.NoError(t, err)
}

func TestSaveSnapshot_WritesExpiredAffiliate(t *testing.T) {
	path := snapshotPath(t)
	s := stateWith(t, "c1", "expired", -time.Second, []byte("old"))

	require.NoError(t, s.SaveSnapshot(path))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0), "expired affiliate should still be written")
}

func TestSaveSnapshot_WritesExpiredEndpoints(t *testing.T) {
	path := snapshotPath(t)
	s := newTestState(t)
	c := s.ClusterFor("c1")

	err := c.UpdateAffiliate(context.Background(), AffiliateUpdateRequest{
		ID:        "node-1",
		TTL:       -time.Second,
		Endpoints: [][]byte{[]byte("10.0.0.1")},
	})
	require.NoError(t, err)

	err = c.UpdateAffiliate(context.Background(), AffiliateUpdateRequest{
		ID:        "node-1",
		TTL:       time.Hour,
		Endpoints: [][]byte{[]byte("10.0.0.2")},
	})
	require.NoError(t, err)

	require.NoError(t, s.SaveSnapshot(path))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0), "expired endpoints should still be written")
}

func TestSaveSnapshot_WritesEmptyCluster(t *testing.T) {
	path := snapshotPath(t)
	s := newTestState(t)
	c := s.ClusterFor("c1")

	err := c.UpdateAffiliate(context.Background(), AffiliateUpdateRequest{
		ID:   "node-1",
		TTL:  time.Hour,
		Data: []byte("data"),
	})
	require.NoError(t, err)
	require.NoError(t, c.DeleteAffiliate(context.Background(), "node-1"))

	require.NoError(t, s.SaveSnapshot(path))

	_, err = os.Stat(path)
	assert.NoError(t, err, "empty cluster should still produce a snapshot file")
}

func TestSaveSnapshot_NoTmpFileAfterSuccess(t *testing.T) {
	path := snapshotPath(t)
	s := stateWith(t, "c1", "node-1", time.Hour, []byte("data"))

	require.NoError(t, s.SaveSnapshot(path))

	_, err := os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err), ".tmp file should be removed after successful save")
}

func TestSaveSnapshot_OverwritesExisting(t *testing.T) {
	path := snapshotPath(t)
	s1 := stateWith(t, "c1", "node-1", time.Hour, []byte("v1"))
	require.NoError(t, s1.SaveSnapshot(path))

	s2 := stateWith(t, "c1", "node-1", time.Hour, []byte("v2"))
	require.NoError(t, s2.SaveSnapshot(path))

	s3, err := NewState(WithSnapshot(path))
	require.NoError(t, err)

	affiliates, err := s3.ClusterFor("c1").ListAffiliates(context.Background())
	require.NoError(t, err)
	require.Len(t, affiliates, 1)
	assert.Equal(t, []byte("v2"), affiliates[0].Data)
}

// --- LoadSnapshot (package-level function) ---

func TestLoadSnapshot_FileNotExist_ReturnsEmptyMap(t *testing.T) {
	clusters, err := LoadSnapshot("/tmp/does-not-exist-talos-discovery.bin")
	require.NoError(t, err)
	assert.NotNil(t, clusters)

	count := 0
	clusters.Range(func(_ string, _ *Cluster) bool {
		count++
		return true
	})
	assert.Equal(t, 0, count)
}

func TestLoadSnapshot_CorruptFile_ReturnsError(t *testing.T) {
	path := snapshotPath(t)
	require.NoError(t, os.WriteFile(path, []byte("not valid protobuf"), 0o600))

	_, err := LoadSnapshot(path)
	assert.Error(t, err)
}

func TestLoadSnapshot_RestoresExpiredAffiliate(t *testing.T) {
	s := stateWith(t, "c1", "expired", -time.Second, []byte("old"))
	s2 := roundTrip(t, s)

	affiliates, err := s2.ClusterFor("c1").ListAffiliates(context.Background())
	require.NoError(t, err)
	assert.Len(t, affiliates, 1, "expired affiliate should be restored — pruning is Cleanup's job")
}

func TestLoadSnapshot_RestoresExpiredEndpoints(t *testing.T) {
	s := newTestState(t)
	c := s.ClusterFor("c1")

	err := c.UpdateAffiliate(context.Background(), AffiliateUpdateRequest{
		ID:        "node-1",
		TTL:       -time.Second,
		Endpoints: [][]byte{[]byte("10.0.0.1")},
	})
	require.NoError(t, err)

	err = c.UpdateAffiliate(context.Background(), AffiliateUpdateRequest{
		ID:        "node-1",
		TTL:       time.Hour,
		Endpoints: [][]byte{[]byte("10.0.0.2")},
	})
	require.NoError(t, err)

	s2 := roundTrip(t, s)

	affiliates, err := s2.ClusterFor("c1").ListAffiliates(context.Background())
	require.NoError(t, err)
	require.Len(t, affiliates, 1)
	assert.Len(t, affiliates[0].Endpoints, 2, "both endpoints should be restored regardless of expiry")
}

// --- round-trip ---

func TestRoundTrip_AffiliateData(t *testing.T) {
	s := stateWith(t, "c1", "node-1", time.Hour, []byte("hello"))
	s2 := roundTrip(t, s)

	affiliates, err := s2.ClusterFor("c1").ListAffiliates(context.Background())
	require.NoError(t, err)
	require.Len(t, affiliates, 1)
	assert.Equal(t, "node-1", affiliates[0].ID)
	assert.Equal(t, []byte("hello"), affiliates[0].Data)
}

func TestRoundTrip_Endpoints(t *testing.T) {
	s := stateWith(t, "c1", "node-1", time.Hour, []byte("data"),
		[]byte("10.0.0.1"), []byte("10.0.0.2"),
	)
	s2 := roundTrip(t, s)

	affiliates, err := s2.ClusterFor("c1").ListAffiliates(context.Background())
	require.NoError(t, err)
	require.Len(t, affiliates, 1)
	assert.Len(t, affiliates[0].Endpoints, 2)
}

func TestRoundTrip_MultipleClusters(t *testing.T) {
	s := newTestState(t)
	for _, id := range []string{"c1", "c2", "c3"} {
		err := s.ClusterFor(id).UpdateAffiliate(context.Background(), AffiliateUpdateRequest{
			ID:   "node-1",
			TTL:  time.Hour,
			Data: []byte(id),
		})
		require.NoError(t, err)
	}

	s2 := roundTrip(t, s)

	for _, id := range []string{"c1", "c2", "c3"} {
		affiliates, err := s2.ClusterFor(id).ListAffiliates(context.Background())
		require.NoError(t, err)
		require.Len(t, affiliates, 1, "cluster %s should have one affiliate", id)
		assert.Equal(t, []byte(id), affiliates[0].Data)
	}
}

func TestRoundTrip_MultipleAffiliates(t *testing.T) {
	s := newTestState(t)
	c := s.ClusterFor("c1")
	for i := 0; i < 5; i++ {
		err := c.UpdateAffiliate(context.Background(), AffiliateUpdateRequest{
			ID:   fmt.Sprintf("node-%d", i),
			TTL:  time.Hour,
			Data: []byte{byte(i)},
		})
		require.NoError(t, err)
	}

	s2 := roundTrip(t, s)

	affiliates, err := s2.ClusterFor("c1").ListAffiliates(context.Background())
	require.NoError(t, err)
	assert.Len(t, affiliates, 5)
}

func TestRoundTrip_PreservesExpiry(t *testing.T) {
	ttl := time.Hour
	s := stateWith(t, "c1", "node-1", ttl, []byte("data"))
	s2 := roundTrip(t, s)

	c, ok := s2.GetCluster("c1")
	require.True(t, ok)

	c.affiliatesMu.Lock()
	aff := c.affiliates["node-1"]
	c.affiliatesMu.Unlock()
	require.NotNil(t, aff)

	remaining := time.Until(aff.expiry)
	assert.Greater(t, remaining, ttl-5*time.Second, "expiry should be preserved across round-trip")
	assert.Less(t, remaining, ttl+5*time.Second)
}

func TestRoundTrip_PreservesExpiredExpiry(t *testing.T) {
	s := stateWith(t, "c1", "node-1", -time.Second, []byte("data"))

	c := s.ClusterFor("c1")
	c.affiliatesMu.Lock()
	originalExpiry := c.affiliates["node-1"].expiry
	c.affiliatesMu.Unlock()

	s2 := roundTrip(t, s)

	c2, ok := s2.GetCluster("c1")
	require.True(t, ok)
	c2.affiliatesMu.Lock()
	restoredExpiry := c2.affiliates["node-1"].expiry
	c2.affiliatesMu.Unlock()

	assert.WithinDuration(t, originalExpiry, restoredExpiry, time.Second)
}

func TestRoundTrip_DoesNotFireSubscriberEvents(t *testing.T) {
	// Subscribe to a cluster before loading the snapshot into it —
	// LoadSnapshot must not trigger any events since it bypasses notify.
	path := snapshotPath(t)
	s := stateWith(t, "c1", "node-1", time.Hour, []byte("data"))
	require.NoError(t, s.SaveSnapshot(path))

	s2 := newTestState(t)
	_, sub, err := s2.ClusterFor("c1").SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)
	defer sub.Close()

	// Load via package-level function — s2.clusters will be replaced.
	clusters, err := LoadSnapshot(path)
	require.NoError(t, err)
	s2.clusters = clusters

	select {
	case ev := <-sub.Events():
		t.Fatalf("unexpected event during LoadSnapshot: %+v", ev)
	case <-time.After(50 * time.Millisecond):
		// expected: no events
	}
}

// --- atomicWriteFile ---

func TestAtomicWriteFile_ContentCorrect(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.bin")
	data := []byte("snapshot content")

	require.NoError(t, atomicWriteFile(path, data, defaultPerm))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestAtomicWriteFile_NoTmpFileAfterSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.bin")

	require.NoError(t, atomicWriteFile(path, []byte("hello"), defaultPerm))

	_, err := os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err), ".tmp file should be gone after successful write")
}

func TestAtomicWriteFile_OverwritesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.bin")
	require.NoError(t, atomicWriteFile(path, []byte("v1"), defaultPerm))
	require.NoError(t, atomicWriteFile(path, []byte("v2"), defaultPerm))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, []byte("v2"), got)
}

func snapshotPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "snapshot.bin")
}

func newTestState(t *testing.T) *State {
	t.Helper()
	s, err := NewState()
	require.NoError(t, err)
	return s
}

func stateWith(t *testing.T, clusterID, affiliateID string, ttl time.Duration, data []byte, endpoints ...[]byte) *State {
	t.Helper()
	s := newTestState(t)
	err := s.ClusterFor(clusterID).UpdateAffiliate(context.Background(), AffiliateUpdateRequest{
		ID:        affiliateID,
		TTL:       ttl,
		Data:      data,
		Endpoints: endpoints,
	})
	require.NoError(t, err)
	return s
}

func roundTrip(t *testing.T, s *State) *State {
	t.Helper()
	path := snapshotPath(t)
	require.NoError(t, s.SaveSnapshot(path))
	s2, err := NewState(WithSnapshot(path))
	require.NoError(t, err)
	return s2
}
