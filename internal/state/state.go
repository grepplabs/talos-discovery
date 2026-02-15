package state

import (
	"github.com/grepplabs/loggo/zlog"
	"github.com/puzpuzpuz/xsync/v4"
)

type StateOption func(*stateConfig)

type stateConfig struct {
	snapshotPath string
}

func WithSnapshot(path string) StateOption {
	return func(c *stateConfig) {
		c.snapshotPath = path
	}
}

type State struct {
	clusters *xsync.Map[string, *Cluster]
}

func NewState(opts ...StateOption) (*State, error) {
	cfg := &stateConfig{}
	for _, o := range opts {
		o(cfg)
	}

	s := &State{
		clusters: xsync.NewMap[string, *Cluster](),
	}

	if cfg.snapshotPath != "" {
		clusters, err := LoadSnapshot(cfg.snapshotPath)
		if err != nil {
			return nil, err
		}
		s.clusters = clusters
	}
	return s, nil
}

// ClusterFor returns the cluster for the given ID, creating it if it does not exist.
func (s *State) ClusterFor(clusterId string) *Cluster {
	cluster, _ := s.clusters.LoadOrCompute(clusterId, func() (*Cluster, bool) {
		zlog.Infow("Cluster created", "id", clusterId)
		return NewCluster(clusterId), false
	})
	return cluster
}

func (s *State) GetCluster(clusterId string) (*Cluster, bool) {
	return s.clusters.Load(clusterId)
}

func (s *State) Cleanup() {
	zlog.Debugw("Cleanup state invoked")
	s.clusters.Range(func(id string, cluster *Cluster) bool {
		cluster.Prune()
		if cluster.IsExpired() && cluster.IsEmpty() {
			s.clusters.Delete(id)
			zlog.Infow("Cluster deleted", "id", id)
		}
		return true
	})
}
