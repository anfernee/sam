package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/spf13/cobra"

	samnet "sam/pkg/net"
	"sam/pkg/protocol"
)

func newProxyCmd(cfg *runConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proxy",
		Short: "Start a local HTTP proxy tunnel over libp2p",
		Long: `Listen on a local HTTP port and forward requests through SAM.

The destination must be set on each request via X-SAM-Target:
  - PeerID: routes directly to a specific peer
  - Capability: discovers the closest provider via DHT and routes there`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runProxy(cmd.Context(), cfg)
		},
	}

	f := cmd.Flags()
	f.IntVar(&cfg.proxyPort, "port", 0, "local HTTP listen port")
	f.StringVar(&cfg.proxyTargetHdr, "target-header", "X-SAM-Target", "request header used to select peer-id or capability")
	f.StringVar(&cfg.proxyBiscuit, "biscuit", "dev-biscuit", "biscuit token forwarded in tunnel metadata")
	f.DurationVar(&cfg.proxyTimeout, "timeout", 30*time.Second, "per-request tunnel timeout")
	_ = cmd.MarkFlagRequired("port")

	return cmd
}

func runProxy(parent context.Context, cfg *runConfig) error {
	if cfg.proxyPort <= 0 {
		return fmt.Errorf("--port must be a positive integer")
	}
	if strings.TrimSpace(cfg.proxyTargetHdr) == "" {
		cfg.proxyTargetHdr = "X-SAM-Target"
	}
	if cfg.proxyTimeout <= 0 {
		cfg.proxyTimeout = 30 * time.Second
	}

	node, err := buildNode(cfg)
	if err != nil {
		return err
	}
	if err := node.Start(parent); err != nil {
		return fmt.Errorf("starting node: %w", err)
	}
	defer func() { _ = node.Stop(context.Background()) }()

	observer, err := protocol.NewBoltObserverForFederation(cfg.federation)
	if err != nil {
		return fmt.Errorf("creating reputation observer: %w", err)
	}
	defer func() { _ = observer.Close() }()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		vouch, vErr := loadLocalVouch()
		if vErr != nil {
			http.Error(w, "unauthorized: local identity login required", http.StatusUnauthorized)
			return
		}

		targetArg := strings.TrimSpace(r.Header.Get(cfg.proxyTargetHdr))
		if targetArg == "" {
			http.Error(w, fmt.Sprintf("missing %s header", cfg.proxyTargetHdr), http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), cfg.proxyTimeout)
		defer cancel()

		target, capability, err := resolveProxyTarget(ctx, node, targetArg)
		if err != nil {
			http.Error(w, fmt.Sprintf("target resolution failed: %v", err), http.StatusBadGateway)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			observer.OnFailure(target.ID.String(), protocol.FailureTypeProtocol)
			http.Error(w, fmt.Sprintf("reading request body: %v", err), http.StatusBadRequest)
			return
		}

		requestHeaders := r.Header.Clone()
		requestHeaders.Del(cfg.proxyTargetHdr)

		tunnelReq := protocol.HTTPTunnelRequest{
			Method:  r.Method,
			Path:    r.URL.RequestURI(),
			Headers: requestHeaders,
			Body:    body,
		}

		if cfg.debug {
			slog.Default().Info("proxy hop", "path", "[Local HTTP] -> ["+target.ID.String()+"] -> [Remote Service]")
		}

		if len(target.Addrs) > 0 {
			_ = node.Connect(ctx, target)
		}

		resp, err := protocol.TunnelHTTP(ctx, node.Host(), target.ID, protocol.HTTPTunnelOpenRequest{
			Vouch:      vouch,
			Biscuit:    strings.TrimSpace(cfg.proxyBiscuit),
			Capability: capability,
			Request:    tunnelReq,
		})
		if err != nil {
			observer.OnFailure(target.ID.String(), protocol.FailureTypeLiveness)
			http.Error(w, fmt.Sprintf("tunnel request failed: %v", err), http.StatusBadGateway)
			return
		}
		if resp.Error != "" {
			observer.OnFailure(target.ID.String(), protocol.FailureTypeRemote)
			status := resp.StatusCode
			if status == 0 {
				status = http.StatusBadGateway
			}
			http.Error(w, resp.Error, status)
			return
		}

		for key, vals := range resp.Headers {
			for _, val := range vals {
				w.Header().Add(key, val)
			}
		}
		status := resp.StatusCode
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = w.Write(resp.Body)

		observer.OnSuccess(target.ID.String(), time.Since(start))
	})

	addr := ":" + strconv.Itoa(cfg.proxyPort)
	srv := &http.Server{Addr: addr, Handler: handler}
	go func() {
		<-parent.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Default().Info("SAM HTTP proxy is up", "peer_id", node.PeerID(), "listen", addr, "target_header", cfg.proxyTargetHdr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("starting local proxy HTTP server: %w", err)
	}
	return nil
}

func resolveProxyTarget(ctx context.Context, node samnet.Node, targetArg string) (peer.AddrInfo, string, error) {
	targetArg = strings.TrimSpace(targetArg)
	if targetArg == "" {
		return peer.AddrInfo{}, "", fmt.Errorf("target peer ID or capability is required")
	}

	if pid, err := peer.Decode(targetArg); err == nil {
		return peer.AddrInfo{ID: pid, Addrs: node.Host().Peerstore().Addrs(pid)}, "", nil
	}

	svc, err := protocol.NewDiscoveryService(node)
	if err != nil {
		return peer.AddrInfo{}, "", fmt.Errorf("creating discovery service: %w", err)
	}
	peers, err := svc.DiscoverPeers(ctx, targetArg)
	if err != nil {
		return peer.AddrInfo{}, "", fmt.Errorf("discovering capability %q: %w", targetArg, err)
	}
	if len(peers) == 0 {
		return peer.AddrInfo{}, "", fmt.Errorf("no peers found for capability %q", targetArg)
	}

	closest := closestPeer(node.PeerID(), peers)
	return closest, targetArg, nil
}

func closestPeer(local peer.ID, peers []peer.AddrInfo) peer.AddrInfo {
	if len(peers) == 1 {
		return peers[0]
	}
	localBytes := []byte(local)
	best := peers[0]
	bestDistance := xorDistance(localBytes, []byte(best.ID))
	for i := 1; i < len(peers); i++ {
		d := xorDistance(localBytes, []byte(peers[i].ID))
		if d.Cmp(bestDistance) < 0 {
			best = peers[i]
			bestDistance = d
		}
	}
	return best
}

func xorDistance(a, b []byte) *big.Int {
	max := len(a)
	if len(b) > max {
		max = len(b)
	}
	buf := make([]byte, max)
	for i := 0; i < max; i++ {
		var av, bv byte
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		buf[i] = av ^ bv
	}
	return new(big.Int).SetBytes(buf)
}
