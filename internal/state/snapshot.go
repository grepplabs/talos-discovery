package state

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/grepplabs/loggo/zlog"
	"github.com/puzpuzpuz/xsync/v4"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	snapshotv1 "github.com/grepplabs/talos-discovery/internal/proto/snapshot/v1"
)

const defaultPerm os.FileMode = 0o600

// SaveSnapshot writes all cluster and affiliate state to path using
// protobuf encoding and an atomic write.
func (s *State) SaveSnapshot(path string) error {
	zlog.Infof("saving state snapshot to: %s", path)

	snap := &snapshotv1.Snapshot{}

	s.clusters.Range(func(id string, cluster *Cluster) bool {
		sc := &snapshotv1.Cluster{Id: id}

		cluster.affiliatesMu.Lock()
		for _, aff := range cluster.affiliates {
			sa := &snapshotv1.Affiliate{
				Id:     aff.id,
				Expiry: timestamppb.New(aff.expiry),
				Data:   aff.data,
			}
			for _, ep := range aff.endpoints {
				sa.Endpoints = append(sa.Endpoints, &snapshotv1.Endpoint{
					Data:   ep.data,
					Expiry: timestamppb.New(ep.expiry),
				})
			}
			sc.Affiliates = append(sc.Affiliates, sa)
		}
		cluster.affiliatesMu.Unlock()

		snap.Clusters = append(snap.Clusters, sc)
		return true
	})

	data, err := proto.Marshal(snap)
	if err != nil {
		return err
	}
	return atomicWriteFile(path, data, defaultPerm)
}

// LoadSnapshot reads a snapshot from path and returns the cluster map.
// If the file does not exist, it returns an empty map.
func LoadSnapshot(path string) (*xsync.Map[string, *Cluster], error) {
	zlog.Infof("loading state snapshot from: %s", path)

	//nolint:gosec // provided in the config
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			zlog.Infof("state snapshot not found: %s", path)
			return xsync.NewMap[string, *Cluster](), nil
		}
		return nil, err
	}

	var snap snapshotv1.Snapshot
	if err := proto.Unmarshal(data, &snap); err != nil {
		return nil, err
	}

	clusters := xsync.NewMap[string, *Cluster]()
	var totalAffiliates, totalEndpoints int

	for _, sc := range snap.GetClusters() {
		cluster := NewCluster(sc.GetId())
		for _, sa := range sc.GetAffiliates() {
			aff := &Affiliate{
				id:   sa.GetId(),
				data: sa.GetData(),
			}
			if sa.GetExpiry() != nil {
				aff.expiry = sa.GetExpiry().AsTime()
			}
			for _, se := range sa.GetEndpoints() {
				ep := Endpoint{data: se.GetData()}
				if se.GetExpiry() != nil {
					ep.expiry = se.GetExpiry().AsTime()
				}
				aff.endpoints = append(aff.endpoints, ep)
				totalEndpoints++
			}
			cluster.affiliates[aff.id] = aff
			totalAffiliates++
		}
		clusters.Store(sc.GetId(), cluster)
	}
	zlog.Infof("loaded state snapshot: clusters=%d affiliates=%d endpoints=%d", len(snap.GetClusters()), totalAffiliates, totalEndpoints)
	return clusters, nil
}

//nolint:unparam,gosec // path provided in the config
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmpFile.Name()

	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
	}()

	if err := tmpFile.Chmod(perm); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	// sync parent directory (ensures rename is persisted)
	dirFile, err := os.Open(dir)
	if err == nil {
		//nolint:errcheck
		defer dirFile.Close()
		_ = dirFile.Sync()
	}

	return nil
}
