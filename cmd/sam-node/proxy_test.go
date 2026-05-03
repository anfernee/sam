package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/sam/api"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestDatapathIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Setup: Create two in-memory nodes (Node A and Node B)
	privA, _, _ := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	privB, _, _ := crypto.GenerateKeyPair(crypto.Ed25519, -1)

	dirA := t.TempDir()
	dirB := t.TempDir()

	storeA, err := NewStore(dirA)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := storeA.Close(); err != nil {
			t.Logf("failed to close storeA: %v", err)
		}
	}()

	storeB, err := NewStore(dirB)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := storeB.Close(); err != nil {
			t.Logf("failed to close storeB: %v", err)
		}
	}()

	// Pre-populate stores with dummy keys to avoid enrollment failure if required
	// For this test, we assume we can run without full enrollment if we bypass AuthHandler

	nodeA, err := NewSamNode(ctx, privA, nil, nil, storeA, "test-mesh", "1s", []string{"/ip4/127.0.0.1/tcp/0"}, false, &CompiledLocalPolicy{}, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := nodeA.Host.Close(); err != nil {
			t.Logf("failed to close nodeA host: %v", err)
		}
	}()

	nodeB, err := NewSamNode(ctx, privB, nil, nil, storeB, "test-mesh", "1s", []string{"/ip4/127.0.0.1/tcp/0"}, false, &CompiledLocalPolicy{}, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := nodeB.Host.Close(); err != nil {
			t.Logf("failed to close nodeB host: %v", err)
		}
	}()

	// Connect Node B to Node A directly
	err = nodeB.Host.Connect(ctx, peer.AddrInfo{ID: nodeA.Host.ID(), Addrs: nodeA.Host.Addrs()})
	if err != nil {
		t.Fatal(err)
	}

	// 2. Target Service: On Node A, start a dummy HTTP server
	expectedHeaderValue := "test-value"
	expectedBody := `{"status":"success"}`

	dummyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test-Header") != expectedHeaderValue {
			t.Errorf("Expected header X-Test-Header to be %s, got %s", expectedHeaderValue, r.Header.Get("X-Test-Header"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(expectedBody))
	}))
	defer dummyServer.Close()

	// 3. Registration: Node A registers this dummy server in its ServiceRegistry
	serviceName := "dummy-tool"
	serviceInfo := &api.ServiceInfo{
		Type: api.ServiceType_SERVICE_TYPE_MCP,
		Name: serviceName,
	}
	
	// We need to make sure the node knows how to handle this service locally.
	// We use the dummy server as the handler.
	nodeA.RegisterServiceHandler(serviceName, dummyServer.Config.Handler)
	
	// We also register it in the DHT so Node B can find it if needed (though we use direct URL in test)
	// We ignore the error because DHT Provide might fail if routing table is empty in this isolated test.
	_ = nodeA.RegisterService(ctx, serviceInfo)
	
	// Manually add to services map to ensure it's registered for the test lookup
	nodeA.servicesMu.Lock()
	nodeA.services[serviceName] = serviceInfo
	nodeA.servicesMu.Unlock()

	// Start the Sidecar server for Node B to get BoundHTTPAddr populated
	// We don't need full sidecar for Node B in this test if we call createEgressProxy directly,
	// but let's populate BoundHTTPAddr to avoid nil panics if used.
	nodeB.BoundHTTPAddr = "127.0.0.1:0" // Dummy

	// 4. Execution: Node B makes a request via its local Egress Proxy
	
	// We need to create the egress proxy for Node B.
	// The user request says: "Implement a reverse proxy on the local `sam-node` HTTP server that intercepts requests to `/sam/`"
	// In sidecar.go we have `createEgressProxy(node)`. We can use it here.
	
	proxyHandler := createEgressProxy(nodeB)
	
	// We start a test server for Node B's proxy to simulate the local agent calling it.
	proxyServer := httptest.NewServer(proxyHandler)
	defer proxyServer.Close()

	// Construct the URL targeting Node A's service
	// http://localhost:<port>/sam/{peer_id}/{service_type}/{service_name}/{upstream_path}
	url := fmt.Sprintf("%s/sam/%s/mcp/%s/api/v1/test", proxyServer.URL, nodeA.Host.ID().String(), serviceName)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Test-Header", expectedHeaderValue)

	client := &http.Client{}
	
	pidStr := nodeA.Host.ID().String()
	_, err = peer.Decode(pidStr)
	if err != nil {
		t.Fatalf("Failed to decode generated peer ID %s: %v", pidStr, err)
	}
	
	// Wait for DHT/routing to settle or just retry a few times if needed.
	// Since we connected directly, it should work immediately if the protocol is registered.
	
	var resp *http.Response
	for i := 0; i < 3; i++ {
		resp, err = client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}
		t.Logf("Attempt %d failed: %v, status: %v", i+1, err, resp)
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("failed to close response body: %v", err)
		}
	}()

	// 5. Assertions
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status OK, got %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if string(bodyBytes) != expectedBody {
		t.Fatalf("Expected body %s, got %s", expectedBody, string(bodyBytes))
	}
}
