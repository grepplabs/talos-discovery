package server

import (
	"context"
	"errors"
	"net/netip"
	"strings"

	"github.com/grepplabs/loggo/zlog"
	discoveryv1alpha1 "github.com/siderolabs/discovery-api/api/v1alpha1/server/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/grepplabs/talos-discovery/internal/config"
	"github.com/grepplabs/talos-discovery/internal/state"
)

type clusterServer struct {
	discoveryv1alpha1.UnimplementedClusterServer

	state            *state.State
	serverStop       <-chan struct{}
	redirectEndpoint string
}

func newClusterServer(ctx context.Context, state *state.State, discoveryConfig config.DiscoveryConfig) *clusterServer {
	return &clusterServer{
		serverStop:       ctx.Done(),
		state:            state,
		redirectEndpoint: discoveryConfig.RedirectEndpoint,
	}
}

func (s *clusterServer) Hello(ctx context.Context, req *discoveryv1alpha1.HelloRequest) (*discoveryv1alpha1.HelloResponse, error) {
	if err := validateClusterID(req.GetClusterId()); err != nil {
		return nil, err
	}
	resp := &discoveryv1alpha1.HelloResponse{}

	// client IP
	clientIP := extractClientIP(ctx)
	if clientIP.IsValid() {
		resp.ClientIp = clientIP.AsSlice()
	}
	zlog.Debugf("service has received peer IP: %s version: %s", clientIP, req.GetClientVersion())

	// redirect endpoint
	if s.redirectEndpoint != "" {
		resp.Redirect = &discoveryv1alpha1.RedirectMessage{
			Endpoint: s.redirectEndpoint,
		}
	}
	return resp, nil
}

func (s *clusterServer) AffiliateUpdate(ctx context.Context, req *discoveryv1alpha1.AffiliateUpdateRequest) (*discoveryv1alpha1.AffiliateUpdateResponse, error) {
	if err := validateClusterID(req.GetClusterId()); err != nil {
		return nil, err
	}
	if err := validateAffiliateID(req.GetAffiliateId()); err != nil {
		return nil, err
	}
	if err := validateAffiliateData(req.GetAffiliateData()); err != nil {
		return nil, err
	}
	if err := validateAffiliateEndpoints(req.GetAffiliateEndpoints()); err != nil {
		return nil, err
	}
	if err := validateAffiliateTTL(req.GetTtl()); err != nil {
		return nil, err
	}
	err := s.state.ClusterFor(req.GetClusterId()).UpdateAffiliate(ctx, state.AffiliateUpdateRequest{
		ID:        req.GetAffiliateId(),
		Data:      req.GetAffiliateData(),
		Endpoints: req.GetAffiliateEndpoints(),
		TTL:       req.GetTtl().AsDuration(),
	})
	if err != nil {
		switch {
		case errors.Is(err, state.ErrTooManyAffiliates), errors.Is(err, state.ErrTooManyEndpoints):
			return nil, status.Error(codes.ResourceExhausted, err.Error())
		default:
			return nil, status.Errorf(codes.Internal, "update affiliate: %v", err)
		}
	}
	return &discoveryv1alpha1.AffiliateUpdateResponse{}, nil
}

func (s *clusterServer) AffiliateDelete(ctx context.Context, req *discoveryv1alpha1.AffiliateDeleteRequest) (*discoveryv1alpha1.AffiliateDeleteResponse, error) {
	if err := validateClusterID(req.GetClusterId()); err != nil {
		return nil, err
	}
	if err := validateAffiliateID(req.GetAffiliateId()); err != nil {
		return nil, err
	}
	if cluster, ok := s.state.GetCluster(req.GetClusterId()); ok {
		if err := cluster.DeleteAffiliate(ctx, req.GetAffiliateId()); err != nil {
			return nil, status.Errorf(codes.Internal, "delete affiliate: %v", err)
		}
	}
	return &discoveryv1alpha1.AffiliateDeleteResponse{}, nil
}

func (s *clusterServer) List(ctx context.Context, req *discoveryv1alpha1.ListRequest) (*discoveryv1alpha1.ListResponse, error) {
	if err := validateClusterID(req.GetClusterId()); err != nil {
		return nil, err
	}
	cluster, ok := s.state.GetCluster(req.GetClusterId())
	if !ok {
		return &discoveryv1alpha1.ListResponse{}, nil
	}

	affiliates, err := cluster.ListAffiliates(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list affiliates: %v", err)
	}
	resp := &discoveryv1alpha1.ListResponse{
		Affiliates: make([]*discoveryv1alpha1.Affiliate, 0, len(affiliates)),
	}
	for i := range affiliates {
		resp.Affiliates = append(resp.Affiliates, toProtoAffiliate(affiliates[i]))
	}
	return resp, nil
}

//nolint:cyclop,funlen
func (s *clusterServer) Watch(
	req *discoveryv1alpha1.WatchRequest,
	server grpc.ServerStreamingServer[discoveryv1alpha1.WatchResponse],
) error {
	if err := validateClusterID(req.GetClusterId()); err != nil {
		return err
	}

	ctx := server.Context()
	cluster := s.state.ClusterFor(req.GetClusterId())

	snapshot, sub, err := cluster.SubscribeWithSnapshot(ctx, extractClientIP(ctx))
	if err != nil {
		return err
	}
	defer sub.Close()

	// send initial snapshot
	if len(snapshot) > 0 {
		resp := &discoveryv1alpha1.WatchResponse{
			Deleted:    false,
			Affiliates: make([]*discoveryv1alpha1.Affiliate, 0, len(snapshot)),
		}
		for i := range snapshot {
			resp.Affiliates = append(resp.Affiliates, toProtoAffiliate(snapshot[i]))
		}
		if err := server.Send(resp); err != nil {
			if status.Code(err) == codes.Canceled {
				return nil
			}
			return err
		}
	}

	// stream incremental changes
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err, ok := <-sub.Errors():
			if ok && err != nil {
				// e.g. "lost update" -> client must reconnect/relist
				return err
			}
			return status.Errorf(codes.Aborted, "subscription canceled: %s", err)

		case ev, ok := <-sub.Events():
			if !ok {
				return nil
			}
			resp := &discoveryv1alpha1.WatchResponse{}
			switch ev.Type {
			case state.AffiliateEventDelete:
				resp.Deleted = true
				resp.Affiliates = []*discoveryv1alpha1.Affiliate{{Id: ev.ID}}

			case state.AffiliateEventUpsert:
				resp.Deleted = false
				resp.Affiliates = []*discoveryv1alpha1.Affiliate{toProtoAffiliate(ev.AffiliateInfo)}
			default:
				continue
			}

			if err := server.Send(resp); err != nil {
				if status.Code(err) == codes.Canceled {
					return nil
				}
				return err
			}
		}
	}
}

// nolint:nestif
func extractClientIP(ctx context.Context) netip.Addr {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-real-ip"); len(vals) > 0 {
			if addr, err := netip.ParseAddr(vals[0]); err == nil {
				return addr
			}
		}
		if vals := md.Get("x-forwarded-for"); len(vals) > 0 {
			ip := strings.TrimSpace(strings.Split(vals[0], ",")[0])
			if addr, err := netip.ParseAddr(ip); err == nil {
				return addr
			}
		}
	}
	if p, ok := peer.FromContext(ctx); ok {
		if addrPort, err := netip.ParseAddrPort(p.Addr.String()); err == nil {
			return addrPort.Addr()
		}
	}
	return netip.Addr{}
}

func toProtoAffiliate(info state.AffiliateInfo) *discoveryv1alpha1.Affiliate {
	return &discoveryv1alpha1.Affiliate{
		Id:        info.ID,
		Data:      info.Data,
		Endpoints: info.Endpoints,
	}
}
