package server

import (
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/grepplabs/talos-discovery/internal/state"
)

var (
	ErrClusterIDEmpty          = status.Error(codes.InvalidArgument, "Cluster ID can't be empty")
	ErrClusterIDTooLong        = status.Error(codes.OutOfRange, "Cluster ID is too long")
	ErrAffiliateIDEmpty        = status.Error(codes.InvalidArgument, "Affiliate ID can't be empty")
	ErrAffiliateTTLEmpty       = status.Error(codes.InvalidArgument, "Affiliate TTL can't be empty")
	ErrAffiliateIDTooLong      = status.Error(codes.OutOfRange, "Affiliate ID is too long")
	ErrAffiliateDataTooBig     = status.Error(codes.OutOfRange, "Affiliate data is too big")
	ErrAffiliateEndpointTooBig = status.Error(codes.OutOfRange, "Affiliate endpoint is too big")
	ErrTTLTooLong              = status.Error(codes.OutOfRange, "TTL is too long")
)

func validateClusterID(clusterId string) error {
	if strings.TrimSpace(clusterId) == "" {
		return ErrClusterIDEmpty
	}
	if len(clusterId) > state.ClusterIDMax {
		return ErrClusterIDTooLong
	}
	return nil
}

func validateAffiliateID(affiliateId string) error {
	if strings.TrimSpace(affiliateId) == "" {
		return ErrAffiliateIDEmpty
	}
	if len(affiliateId) > state.AffiliateIDMax {
		return ErrAffiliateIDTooLong
	}
	return nil
}

func validateAffiliateData(data []byte) error {
	if len(data) > state.AffiliateDataMax {
		return ErrAffiliateDataTooBig
	}
	return nil
}

func validateAffiliateEndpoints(endpoints [][]byte) error {
	for _, endpoint := range endpoints {
		if len(endpoint) > state.AffiliateEndpointMax {
			return ErrAffiliateEndpointTooBig
		}
	}
	return nil
}

func validateAffiliateTTL(ttl *durationpb.Duration) error {
	if ttl == nil {
		return ErrAffiliateTTLEmpty
	}
	return validateTTL(ttl.AsDuration())
}

func validateTTL(ttl time.Duration) error {
	if ttl > state.TTLMax {
		return ErrTTLTooLong
	}
	return nil
}
