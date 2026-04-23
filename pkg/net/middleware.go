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

package samnet

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	coreprotocol "github.com/libp2p/go-libp2p/core/protocol"

	"sam/pkg/economy"
	"sam/pkg/identity"
)

const (
	a2aProtocolID = "/sam/a2a/1.0.0"
	mcpProtocolID = "/sam/mcp/1.0.0"
)

// Middleware wraps stream handlers registered on a host.
type Middleware func(h host.Host, pid coreprotocol.ID, next network.StreamHandler) network.StreamHandler

// AuthMiddlewareConfig controls global authn/authz behavior for all registered protocols.
type AuthMiddlewareConfig struct {
	// AttemptAuthWhenMissing attempts /sam/auth/1.0.0 when a peer is not yet validated.
	AttemptAuthWhenMissing bool
	// ProtocolPolicies stores Datalog-like required facts per protocol, for example:
	// "/sam/a2a/1.0.0" -> {"mesh_membership":"finance"}
	ProtocolPolicies map[string]map[string]string
}

// WrapHost returns a host that applies middleware to every subsequently registered stream handler.
func WrapHost(h host.Host, middlewares ...Middleware) host.Host {
	if h == nil || len(middlewares) == 0 {
		return h
	}
	return &middlewareHost{Host: h, middlewares: middlewares}
}

type middlewareHost struct {
	host.Host
	middlewares []Middleware
}

func (h *middlewareHost) SetStreamHandler(pid coreprotocol.ID, handler network.StreamHandler) {
	wrapped := handler
	for i := len(h.middlewares) - 1; i >= 0; i-- {
		wrapped = h.middlewares[i](h, pid, wrapped)
	}
	h.Host.SetStreamHandler(pid, wrapped)
}

func (h *middlewareHost) SetStreamHandlerMatch(pid coreprotocol.ID, m func(coreprotocol.ID) bool, handler network.StreamHandler) {
	wrapped := handler
	for i := len(h.middlewares) - 1; i >= 0; i-- {
		wrapped = h.middlewares[i](h, pid, wrapped)
	}
	h.Host.SetStreamHandlerMatch(pid, m, wrapped)
}

// NewAuthMiddleware creates a protocol-independent auth middleware that enforces
// authenticated connections and optional claim-based authorization policies.
func NewAuthMiddleware(cfg AuthMiddlewareConfig) Middleware {
	return func(h host.Host, pid coreprotocol.ID, next network.StreamHandler) network.StreamHandler {
		return func(stream network.Stream) {
			protocolName := string(pid)
			if bypassAuthMiddleware(pid) {
				next(stream)
				return
			}

			if err := identity.EnsurePassportAuth(h, "default"); err != nil {
				writeUnauthorizedAndReset(stream, protocolName, fmt.Sprintf("failed to initialize auth middleware: %v", err))
				return
			}

			remote := stream.Conn().RemotePeer()
			claims, err := identity.AuthenticatedPeerPassport(h, remote)
			if err != nil && cfg.AttemptAuthWhenMissing {
				if authErr := identity.AuthenticatePeerPassport(context.Background(), h, remote); authErr == nil {
					claims, err = identity.AuthenticatedPeerPassport(h, remote)
				} else {
					err = authErr
				}
			}
			if err != nil {
				writeUnauthorizedAndReset(stream, protocolName, fmt.Sprintf("passport auth required: %v", err))
				return
			}

			if policyErr := economy.EvaluateProtocolPolicies(protocolName, claims.Claims, cfg.ProtocolPolicies); policyErr != nil {
				writeUnauthorizedAndReset(stream, protocolName, policyErr.Error())
				return
			}

			next(stream)
		}
	}
}

func bypassAuthMiddleware(pid coreprotocol.ID) bool {
	p := string(pid)
	if p == string(identity.PassportAuthProtocolID) {
		return true
	}
	return !strings.HasPrefix(p, "/sam/")
}

func writeUnauthorizedAndReset(stream network.Stream, protocolName, reason string) {
	resp := map[string]string{
		"error":    "Unauthorized",
		"code":     "unauthorized",
		"protocol": protocolName,
		"reason":   strings.TrimSpace(reason),
	}

	switch protocolName {
	case a2aProtocolID, mcpProtocolID:
		_ = json.NewEncoder(stream).Encode(resp)
		_ = stream.Close()
		return
	}

	_ = stream.Reset()
}
