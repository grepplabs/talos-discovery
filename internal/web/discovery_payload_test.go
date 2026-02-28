package web

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	pb "github.com/siderolabs/discovery-api/api/v1alpha1/server/pb"
	"github.com/stretchr/testify/require"
)

func TestAffiliatesToJSON(t *testing.T) {
	out, err := affiliatesToJSON([]*pb.Affiliate{
		{
			Id:        "node-1",
			Data:      []byte("payload"),
			Endpoints: [][]byte{[]byte("ep1"), []byte("ep2")},
		},
		{
			Id:   "node-2",
			Data: nil,
		},
	}, true)
	require.NoError(t, err)

	var env affiliatesEnvelope
	require.NoError(t, json.Unmarshal([]byte(out), &env))

	require.True(t, env.Deleted)
	require.Len(t, env.Affiliates, 2)
	require.Equal(t, "node-1", env.Affiliates[0].ID)
	require.Equal(t, base64.StdEncoding.EncodeToString([]byte("payload")), env.Affiliates[0].Data)
	require.Equal(t, []string{
		base64.StdEncoding.EncodeToString([]byte("ep1")),
		base64.StdEncoding.EncodeToString([]byte("ep2")),
	}, env.Affiliates[0].Endpoints)

	require.Equal(t, "node-2", env.Affiliates[1].ID)
	require.Equal(t, "", env.Affiliates[1].Data)
	require.Equal(t, []string{}, env.Affiliates[1].Endpoints)
}
