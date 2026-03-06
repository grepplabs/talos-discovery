FROM --platform=${BUILDPLATFORM} golang:1.26.1 AS builder
ARG TARGETOS
ARG TARGETARCH
ARG VERSION="unset"

WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a \
        -ldflags="\
          -s -w \
          -X 'github.com/grepplabs/talos-discovery/internal/config.Version=${VERSION}' \
      " \
    -o discovery-service cmd/discovery-service/main.go

FROM --platform=${BUILDPLATFORM} gcr.io/distroless/static-debian13:nonroot
WORKDIR /
COPY --from=builder /workspace/discovery-service .
USER 65532:65532

ENTRYPOINT ["/discovery-service"]
