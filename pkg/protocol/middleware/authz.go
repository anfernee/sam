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

package middleware

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	"sam/pkg/economy"
	"sam/pkg/identity"
)

// ConnectionAuthz is a reusable per-connection authorization hook that can be
// shared across protocol implementations.
type ConnectionAuthz interface {
	Install(host.Host) error
	AuthorizeOutbound(context.Context, host.Host, peer.ID) error
	AuthorizeInbound(context.Context, host.Host, peer.ID) (*identity.PassportClaims, error)
	CheckCapability(context.Context, string, string, string) error
}

// PassportAuthz enforces hub-issued passport authentication on libp2p
// connections. This is the default SAM connection authz middleware.
type PassportAuthz struct{}

func (PassportAuthz) Install(h host.Host) error {
	if h == nil {
		return fmt.Errorf("host is nil")
	}
	if err := identity.EnsurePassportAuth(h, ""); err != nil {
		return fmt.Errorf("installing passport auth: %w", err)
	}
	return nil
}

func (PassportAuthz) AuthorizeOutbound(ctx context.Context, h host.Host, target peer.ID) error {
	if h == nil {
		return fmt.Errorf("host is nil")
	}
	if target == "" {
		return fmt.Errorf("target peer id is required")
	}
	if err := identity.AuthenticatePeerPassport(ctx, h, target); err != nil {
		return fmt.Errorf("passport authentication failed for %s: %w", target, err)
	}
	return nil
}

func (PassportAuthz) AuthorizeInbound(ctx context.Context, h host.Host, remote peer.ID) (*identity.PassportClaims, error) {
	if h == nil {
		return nil, fmt.Errorf("host is nil")
	}
	if remote == "" {
		return nil, fmt.Errorf("remote peer id is required")
	}
	claims, err := identity.EnsureAuthenticatedPeer(ctx, h, remote)
	if err != nil {
		return nil, fmt.Errorf("passport authentication required: %w", err)
	}
	return claims, nil
}

// CheckCapability enforces action/resource caveats embedded in Biscuit tokens.
// If the token does not carry action/resource restrictions, the request is allowed.
func (PassportAuthz) CheckCapability(ctx context.Context, token, action, resource string) error {
	if err := economy.CheckCapability(ctx, nil, token, action, resource); err != nil {
		return fmt.Errorf("authorization denied for action=%q resource=%q: %w", action, resource, err)
	}
	return nil
}
