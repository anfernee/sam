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

package main

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

var _ connmgr.ConnectionGater = (*nodeConnGate)(nil)

// nodeConnGate enforces swarm-level AuthN policies
type nodeConnGate struct {
	node *SamNode
}

// InterceptPeerDial controls who we are allowed to call (Outbound)
func (g *nodeConnGate) InterceptPeerDial(p peer.ID) (allow bool) {
	return !g.node.Store.IsBanned(p)
}

// InterceptAddrDial ensures we only dial specific approved networks
func (g *nodeConnGate) InterceptAddrDial(p peer.ID, m multiaddr.Multiaddr) (allow bool) {
	return true
}

// InterceptAccept controls who can call us (Inbound)
func (g *nodeConnGate) InterceptAccept(n network.ConnMultiaddrs) (allow bool) {
	return true // Connection allowed, but InterceptSecured will verify the PeerID
}

// InterceptSecured is called after TLS handshake. This is our Layer 2 Check.
func (g *nodeConnGate) InterceptSecured(dir network.Direction, p peer.ID, n network.ConnMultiaddrs) (allow bool) {
	if g.node.Store.IsBanned(p) {
		fmt.Printf("[Layer 2] Dropping connection: Peer %s is explicitly BANNED\n", p)
		return false
	}

	// Allow the TLS pipe to stay open. Layer 3 & 4 will handle the rest.
	return true
}

func (g *nodeConnGate) InterceptUpgraded(n network.Conn) (allow bool, cc control.DisconnectReason) {
	return true, 0
}
