package state

import "time"

const (
	TTLMax                = 30 * time.Minute
	ClusterTTL            = TTLMax
	ClusterIDMax          = 256
	AffiliateIDMax        = 256
	AffiliateDataMax      = 2048
	AffiliateEndpointMax  = 32
	AffiliatesMax         = 1024
	AffiliateEndpointsMax = 64
)
