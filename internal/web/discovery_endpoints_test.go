package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	pb "github.com/siderolabs/discovery-api/api/v1alpha1/server/pb"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNewWebServerRequiresClientConfiguration(t *testing.T) {
	_, err := NewWebServer(context.Background(), prometheus.NewRegistry())
	require.ErrorContains(t, err, "missing discovery client configuration")
}

func TestGRPCErrorToHTTPStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "invalid argument", err: status.Error(codes.InvalidArgument, "bad"), want: http.StatusBadRequest},
		{name: "unauthenticated", err: status.Error(codes.Unauthenticated, "no auth"), want: http.StatusUnauthorized},
		{name: "forbidden", err: status.Error(codes.PermissionDenied, "denied"), want: http.StatusForbidden},
		{name: "not found", err: status.Error(codes.NotFound, "missing"), want: http.StatusNotFound},
		{name: "conflict", err: status.Error(codes.AlreadyExists, "exists"), want: http.StatusConflict},
		{name: "too many requests", err: status.Error(codes.ResourceExhausted, "quota"), want: http.StatusTooManyRequests},
		{name: "gateway timeout", err: status.Error(codes.DeadlineExceeded, "slow"), want: http.StatusGatewayTimeout},
		{name: "not implemented", err: status.Error(codes.Unimplemented, "nyi"), want: http.StatusNotImplemented},
		{name: "unavailable", err: status.Error(codes.Unavailable, "down"), want: http.StatusServiceUnavailable},
		{name: "internal", err: status.Error(codes.Internal, "boom"), want: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, grpcErrorToHTTPStatus(tt.err))
		})
	}
}

func TestListAffiliatesHandlerSuccessAndAliases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	client := newBufconnDiscoveryClient(t, &testClusterServer{
		listFunc: func(context.Context, *pb.ListRequest) (*pb.ListResponse, error) {
			return &pb.ListResponse{
				Affiliates: []*pb.Affiliate{
					{
						Id:        "node-1",
						Data:      []byte("data"),
						Endpoints: [][]byte{[]byte("ep-1")},
					},
				},
			}, nil
		},
	})

	addDiscoveryEndpoints(engine, client, &DiscoveryWatchManager{})

	req := httptest.NewRequest(http.MethodGet, "/api/clusters/c1/affiliates", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Affiliates []affiliateJSON `json:"affiliates"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Affiliates, 1)
	require.Equal(t, "node-1", body.Affiliates[0].ID)
	require.Equal(t, base64.StdEncoding.EncodeToString([]byte("data")), body.Affiliates[0].Data)
	require.Equal(t, []string{base64.StdEncoding.EncodeToString([]byte("ep-1"))}, body.Affiliates[0].Endpoints)
}

func TestListAffiliatesHandlerMapsGRPCErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	client := newBufconnDiscoveryClient(t, &testClusterServer{
		listFunc: func(context.Context, *pb.ListRequest) (*pb.ListResponse, error) {
			return nil, status.Error(codes.InvalidArgument, "bad cluster id")
		},
	})

	addDiscoveryEndpoints(engine, client, &DiscoveryWatchManager{})

	req := httptest.NewRequest(http.MethodGet, "/api/clusters/c1/affiliates", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
