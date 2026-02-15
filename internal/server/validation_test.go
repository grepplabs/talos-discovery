package server

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/grepplabs/talos-discovery/internal/state"
)

func TestValidateClusterID_Valid(t *testing.T) {
	require.NoError(t, validateClusterID("cluster-1"))
}

func TestValidateClusterID_AtMaxLength(t *testing.T) {
	id := strings.Repeat("a", state.ClusterIDMax)
	require.NoError(t, validateClusterID(id))
}

func TestValidateClusterID_Empty(t *testing.T) {
	err := validateClusterID("")
	require.ErrorIs(t, err, ErrClusterIDEmpty)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestValidateClusterID_Whitespace(t *testing.T) {
	for _, id := range []string{" ", "\t", "\n", "   "} {
		err := validateClusterID(id)
		require.ErrorIs(t, err, ErrClusterIDEmpty, "whitespace-only %q should be treated as empty", id)
	}
}

func TestValidateClusterID_TooLong(t *testing.T) {
	id := strings.Repeat("a", state.ClusterIDMax+1)
	err := validateClusterID(id)
	require.ErrorIs(t, err, ErrClusterIDTooLong)
	assert.Equal(t, codes.OutOfRange, grpcCode(err))
}

func TestValidateAffiliateID_Valid(t *testing.T) {
	require.NoError(t, validateAffiliateID("node-1"))
}

func TestValidateAffiliateID_AtMaxLength(t *testing.T) {
	id := strings.Repeat("a", state.AffiliateIDMax)
	require.NoError(t, validateAffiliateID(id))
}

func TestValidateAffiliateID_Empty(t *testing.T) {
	err := validateAffiliateID("")
	require.ErrorIs(t, err, ErrAffiliateIDEmpty)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestValidateAffiliateID_Whitespace(t *testing.T) {
	for _, id := range []string{" ", "\t", "\n", "   "} {
		err := validateAffiliateID(id)
		require.ErrorIs(t, err, ErrAffiliateIDEmpty, "whitespace-only %q should be treated as empty", id)
	}
}

func TestValidateAffiliateID_TooLong(t *testing.T) {
	id := strings.Repeat("a", state.AffiliateIDMax+1)
	err := validateAffiliateID(id)
	require.ErrorIs(t, err, ErrAffiliateIDTooLong)
	assert.Equal(t, codes.OutOfRange, grpcCode(err))
}

func TestValidateAffiliateData_Valid(t *testing.T) {
	require.NoError(t, validateAffiliateData([]byte("some-data")))
}

func TestValidateAffiliateData_Nil(t *testing.T) {
	require.NoError(t, validateAffiliateData(nil))
}

func TestValidateAffiliateData_Empty(t *testing.T) {
	require.NoError(t, validateAffiliateData([]byte{}))
}

func TestValidateAffiliateData_AtMaxSize(t *testing.T) {
	data := make([]byte, state.AffiliateDataMax)
	require.NoError(t, validateAffiliateData(data))
}

func TestValidateAffiliateData_TooBig(t *testing.T) {
	data := make([]byte, state.AffiliateDataMax+1)
	err := validateAffiliateData(data)
	require.ErrorIs(t, err, ErrAffiliateDataTooBig)
	assert.Equal(t, codes.OutOfRange, grpcCode(err))
}

func TestValidateAffiliateEndpoints_Valid(t *testing.T) {
	endpoints := [][]byte{[]byte("10.0.0.1"), []byte("10.0.0.2")}
	require.NoError(t, validateAffiliateEndpoints(endpoints))
}

func TestValidateAffiliateEndpoints_Nil(t *testing.T) {
	require.NoError(t, validateAffiliateEndpoints(nil))
}

func TestValidateAffiliateEndpoints_Empty(t *testing.T) {
	require.NoError(t, validateAffiliateEndpoints([][]byte{}))
}

func TestValidateAffiliateEndpoints_AtMaxSize(t *testing.T) {
	ep := make([]byte, state.AffiliateEndpointMax)
	require.NoError(t, validateAffiliateEndpoints([][]byte{ep}))
}

func TestValidateAffiliateEndpoints_OneTooBig(t *testing.T) {
	endpoints := [][]byte{
		make([]byte, state.AffiliateEndpointMax),   // valid
		make([]byte, state.AffiliateEndpointMax+1), // over limit
	}
	err := validateAffiliateEndpoints(endpoints)
	require.ErrorIs(t, err, ErrAffiliateEndpointTooBig)
	assert.Equal(t, codes.OutOfRange, grpcCode(err))
}

func TestValidateAffiliateEndpoints_FirstTooBig(t *testing.T) {
	// Ensures validation short-circuits on the first offending endpoint.
	endpoints := [][]byte{
		make([]byte, state.AffiliateEndpointMax+1),
		make([]byte, state.AffiliateEndpointMax+1),
	}
	err := validateAffiliateEndpoints(endpoints)
	require.ErrorIs(t, err, ErrAffiliateEndpointTooBig)
}

func TestValidateAffiliateTTL_Valid(t *testing.T) {
	require.NoError(t, validateAffiliateTTL(durationpb.New(time.Minute)))
}

func TestValidateAffiliateTTL_AtMax(t *testing.T) {
	require.NoError(t, validateAffiliateTTL(durationpb.New(state.TTLMax)))
}

func TestValidateAffiliateTTL_Nil(t *testing.T) {
	err := validateAffiliateTTL(nil)
	require.ErrorIs(t, err, ErrAffiliateTTLEmpty)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestValidateAffiliateTTL_TooLong(t *testing.T) {
	err := validateAffiliateTTL(durationpb.New(state.TTLMax + time.Nanosecond))
	require.ErrorIs(t, err, ErrTTLTooLong)
	assert.Equal(t, codes.OutOfRange, grpcCode(err))
}

func TestValidateTTL_Valid(t *testing.T) {
	require.NoError(t, validateTTL(time.Minute))
}

func TestValidateTTL_Zero(t *testing.T) {
	require.NoError(t, validateTTL(0))
}

func TestValidateTTL_AtMax(t *testing.T) {
	require.NoError(t, validateTTL(state.TTLMax))
}

func TestValidateTTL_TooLong(t *testing.T) {
	err := validateTTL(state.TTLMax + time.Nanosecond)
	require.ErrorIs(t, err, ErrTTLTooLong)
	assert.Equal(t, codes.OutOfRange, grpcCode(err))
}

// grpcCode extracts the gRPC status code from an error for readable assertions.
func grpcCode(err error) codes.Code {
	return status.Code(err)
}
