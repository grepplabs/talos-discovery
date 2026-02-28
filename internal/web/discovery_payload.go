package web

import (
	"encoding/base64"
	"encoding/json"

	pb "github.com/siderolabs/discovery-api/api/v1alpha1/server/pb"
)

type affiliateJSON struct {
	ID        string   `json:"id"`
	Data      string   `json:"data"`
	Endpoints []string `json:"endpoints"`
}

type affiliatesEnvelope struct {
	Affiliates []affiliateJSON `json:"affiliates"`
	Deleted    bool            `json:"deleted"`
}

func affiliatesToJSON(affiliates []*pb.Affiliate, deleted bool) (string, error) {
	out := affiliatesEnvelope{
		Affiliates: make([]affiliateJSON, 0, len(affiliates)),
		Deleted:    deleted,
	}
	for _, a := range affiliates {
		aj := affiliateJSON{
			ID:   a.Id,
			Data: base64.StdEncoding.EncodeToString(a.Data),
		}
		if len(a.Endpoints) > 0 {
			aj.Endpoints = make([]string, len(a.Endpoints))
			for i, ep := range a.Endpoints {
				aj.Endpoints[i] = base64.StdEncoding.EncodeToString(ep)
			}
		} else {
			aj.Endpoints = []string{}
		}
		out.Affiliates = append(out.Affiliates, aj)
	}

	b, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
