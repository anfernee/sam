// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/multiformats/go-multiaddr"
	"google.golang.org/protobuf/encoding/protojson"
)

// FetchConfig fetches the hub configuration from the given URL.
func FetchConfig(ctx context.Context, hubBaseURL string) (ed25519.PublicKey, []multiaddr.Multiaddr, error) {
	configURL := strings.TrimRight(hubBaseURL, "/") + "/api/v1/config"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, configURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("creating hub config request: %w", err)
	}

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("requesting hub config: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("hub config returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("reading hub config body: %w", err)
	}

	var cfg HubConfig
	if err := protojson.Unmarshal(body, &cfg); err != nil {
		return nil, nil, fmt.Errorf("decoding hub config response: %w", err)
	}

	pubBytes, err := hex.DecodeString(strings.TrimSpace(cfg.PublicKeyHex))
	if err != nil {
		return nil, nil, fmt.Errorf("decoding hub public key: %w", err)
	}
	if len(pubBytes) != ed25519.PublicKeySize {
		return nil, nil, fmt.Errorf("invalid hub public key length: %d", len(pubBytes))
	}

	var hubAddrs []multiaddr.Multiaddr
	for _, raw := range cfg.BootstrapNodes {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		ma, err := multiaddr.NewMultiaddr(raw)
		if err != nil {
			continue
		}
		hubAddrs = append(hubAddrs, ma)
	}
	if len(hubAddrs) == 0 {
		return nil, nil, fmt.Errorf("hub config did not provide valid bootstrap addresses")
	}

	return ed25519.PublicKey(pubBytes), hubAddrs, nil
}

// FetchPeers fetches the enrolled peers from the hub.
func FetchPeers(ctx context.Context, hubBaseURL string, token string) (*PeerRegistry, error) {
	peersURL := strings.TrimRight(hubBaseURL, "/") + "/api/v1/peers"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, peersURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating hub peers request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting hub peers: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hub peers returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading hub peers body: %w", err)
	}

	var registry PeerRegistry
	if err := protojson.Unmarshal(body, &registry); err != nil {
		return nil, fmt.Errorf("decoding hub peers response: %w", err)
	}

	return &registry, nil
}
