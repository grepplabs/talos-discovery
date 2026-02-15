#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ---- config ----
HOST="${HOST:-localhost}"
TEST_SERVER_PORT="${TEST_SERVER_PORT:-3000}"
IMPORT_PATH="${IMPORT_PATH:-${SCRIPT_DIR}/../../api}"
PROTO_FILE="${PROTO_FILE:-cluster.proto}"
METHOD="${METHOD:-sidero.discovery.server.Cluster/AffiliateUpdate}"

# ---- constraints (upper bounds only) ----
TTL_MAX=1800
CLUSTER_ID_MAX=256
AFFILIATE_ID_MAX=256
AFFILIATE_DATA_MAX=2048
AFFILIATE_ENDPOINT_MAX=32
AFFILIATE_ENDPOINTS_MAX=64
AFFILIATES_MAX=1024

rand_int() { echo $(( RANDOM % ($1 + 1) )); }
rand_range() { echo $(( RANDOM % ($2 - $1 + 1) + $1 )); }

rand_alnum() {
  head -c 4096 /dev/urandom | tr -dc 'A-Za-z0-9_-'
}

rand_alnum_n() {
  rand_alnum | head -c "$1"
}

rand_b64_bytes() {
  head -c "$1" /dev/urandom | base64 | tr -d '\n'
}

# ---- random cluster ----
cluster_id_len=$(rand_range 1 "$CLUSTER_ID_MAX")
cluster_id="$(rand_alnum_n "$cluster_id_len")"

# ---- number of affiliates ----
affiliates_count=$(rand_int "$AFFILIATES_MAX")

echo "Generating $affiliates_count affiliates for cluster $cluster_id"

for ((a=0; a<affiliates_count; a++)); do
  affiliate_id_len=$(rand_range 1 "$AFFILIATE_ID_MAX")
  affiliate_data_len=$(rand_int "$AFFILIATE_DATA_MAX")
  endpoints_count=$(rand_int "$AFFILIATE_ENDPOINTS_MAX")
  ttl_seconds=$(rand_range 1 "$TTL_MAX")

  affiliate_id="$(rand_alnum_n "$affiliate_id_len")"
  affiliate_data="$(rand_b64_bytes "$affiliate_data_len")"
  ttl="1800s"

  # endpoints
  endpoints="[]"
  if (( endpoints_count > 0 )); then
    endpoints="["
    for ((i=0; i<endpoints_count; i++)); do
      ep_len=$(rand_range 1 "$AFFILIATE_ENDPOINT_MAX")
      ep="$(rand_b64_bytes "$ep_len")"
      (( i > 0 )) && endpoints+=", "
      endpoints+="\"$ep\""
    done
    endpoints+="]"
  fi

  payload="$(cat <<JSON
{
  "clusterId": "$cluster_id",
  "affiliateId": "$affiliate_id",
  "affiliateData": "$affiliate_data",
  "ttl": "$ttl",
  "affiliateEndpoints": $endpoints
}
JSON
)"

  echo "$payload"

  echo "$payload" | grpcurl -plaintext \
    -import-path "$IMPORT_PATH" \
    -proto "$PROTO_FILE" \
    -d @ \
    "$HOST:$TEST_SERVER_PORT" \
    "$METHOD"

done
