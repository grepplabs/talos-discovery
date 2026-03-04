package web

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grepplabs/talos-discovery/internal/web/assets"
	"github.com/siderolabs/discovery-api/api/v1alpha1/server/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func addDiscoveryEndpoints(engine *gin.Engine, client *DiscoveryClient, watchManager *DiscoveryWatchManager) *gin.Engine {
	engine.GET("/", func(c *gin.Context) {
		serveEmbedded(c, "index.html", "text/html; charset=utf-8")
	})
	engine.GET("/index.html", func(c *gin.Context) {
		serveEmbedded(c, "index.html", "text/html; charset=utf-8")
	})
	engine.GET("/inspect.html", func(c *gin.Context) {
		serveEmbedded(c, "inspect.html", "text/html; charset=utf-8")
	})
	engine.GET("/styles.css", func(c *gin.Context) {
		serveEmbedded(c, "styles.css", "text/css; charset=utf-8")
	})

	engine.GET("/api/clusters/:id/affiliates", listAffiliatesHandler(client))
	engine.GET("/api/clusters/:id/watch", watchAffiliatesHandler(watchManager))

	return engine
}

func listAffiliatesHandler(client *DiscoveryClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID := c.Param("id")
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		resp, err := client.client.List(ctx, &pb.ListRequest{ClusterId: clusterID})
		if err != nil {
			c.JSON(grpcErrorToHTTPStatus(err), gin.H{"error": err.Error()})
			return
		}
		out := make([]affiliateJSON, 0, len(resp.GetAffiliates()))
		for _, a := range resp.GetAffiliates() {
			eps := make([]string, len(a.GetEndpoints()))
			for i, ep := range a.GetEndpoints() {
				eps[i] = base64.StdEncoding.EncodeToString(ep)
			}
			out = append(out, affiliateJSON{
				ID:        a.GetId(),
				Data:      base64.StdEncoding.EncodeToString(a.GetData()),
				Endpoints: eps,
			})
		}
		c.JSON(http.StatusOK, gin.H{"affiliates": out})
	}
}

func watchAffiliatesHandler(watchManager *DiscoveryWatchManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID := c.Param("id")

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")

		ch := watchManager.Subscribe(clusterID)
		defer watchManager.Unsubscribe(clusterID, ch)

		notify := c.Request.Context().Done()
		pingTicker := time.NewTicker(25 * time.Second)
		defer pingTicker.Stop()
		c.Stream(func(w io.Writer) bool {
			select {
			case <-notify:
				return false
			case evt := <-ch:
				c.SSEvent("update", evt.Data)
				return true
			case <-pingTicker.C:
				c.SSEvent("ping", "")
				return true
			}
		})
	}
}

//nolint:exhaustive,cyclop
func grpcErrorToHTTPStatus(err error) int {
	switch status.Code(err) {
	case codes.InvalidArgument, codes.FailedPrecondition, codes.OutOfRange:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists, codes.Aborted:
		return http.StatusConflict
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.Canceled, codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func serveEmbedded(c *gin.Context, name, contentType string) {
	b, err := assets.FS.ReadFile(name)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Data(http.StatusOK, contentType, b)
}
