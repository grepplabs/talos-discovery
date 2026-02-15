package web

import (
	"context"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestUIIndexPageLoadsInBrowser(t *testing.T) {
	if os.Getenv("CHROMEDP_E2E") != "1" {
		t.Skip("set CHROMEDP_E2E=1 to run browser test")
	}

	browserPath := findBrowserBinary()
	if browserPath == "" {
		t.Skip("no Chrome/Chromium binary found in PATH")
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()

	client := newBufconnDiscoveryClient(t, &testClusterServer{})
	manager, err := NewDiscoveryWatchManager(context.Background(), client, prometheus.NewRegistry())
	require.NoError(t, err)
	addDiscoveryEndpoints(engine, client, manager)

	srv := httptest.NewServer(engine)
	defer srv.Close()

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.ExecPath(browserPath),
			chromedp.Headless,
			chromedp.DisableGPU,
			chromedp.NoSandbox,
			chromedp.Flag("disable-dev-shm-usage", true),
			chromedp.NoFirstRun,
			chromedp.NoDefaultBrowserCheck,
		)...,
	)
	defer cancelAlloc()

	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()

	ctx, cancelTimeout := context.WithTimeout(ctx, 10*time.Second)
	defer cancelTimeout()

	var title string
	err = chromedp.Run(ctx,
		chromedp.Navigate(srv.URL+"/index.html"),
		chromedp.WaitVisible(`#clusterInput`, chromedp.ByQuery),
		chromedp.Title(&title),
	)
	require.NoError(t, err)
	require.Equal(t, "Talos Discovery Service", title)
}

func findBrowserBinary() string {
	for _, name := range []string{
		"google-chrome",
		"chromium",
		"chromium-browser",
		"chrome",
	} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}
