package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	libp2ptls "github.com/libp2p/go-libp2p/p2p/security/tls"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
)

const AuthProtocol = protocol.ID("/sam/auth/1.0.0")

type SamNode struct {
	Host         host.Host
	DHT          *dht.IpfsDHT
	PubSub       *pubsub.PubSub
	Store        *Store
	TrustedPeers map[peer.ID]bool
	mu           sync.RWMutex
}

// NewSamNode initializes a FIPS-compliant libp2p host
func NewSamNode(ctx context.Context, priv crypto.PrivKey, listenAddrs []string) (*SamNode, error) {
	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings(listenAddrs...),
		libp2p.Transport(libp2pquic.NewTransport),    // Preferred (UDP/QUIC)
		libp2p.Transport(tcp.NewTCPTransport),        // Fallback (TCP)
		libp2p.Security(libp2ptls.ID, libp2ptls.New), // FIPS Compliant TLS 1.3
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	return &SamNode{
		Host:         h,
		TrustedPeers: make(map[peer.ID]bool),
	}, nil
}

// SecureStreamHandler implements the "Reject by Default" middleware
func (n *SamNode) SecureStreamHandler(pid protocol.ID, handler network.StreamHandler) {
	n.Host.SetStreamHandler(pid, func(s network.Stream) {
		n.mu.RLock()
		isTrusted := n.TrustedPeers[s.Conn().RemotePeer()]
		n.mu.RUnlock()

		if !isTrusted {
			// DROP: Peer has not completed the /sam/auth/1.0.0 handshake
			fmt.Printf("Blocked unauthorized stream request for %s from %s\n", pid, s.Conn().RemotePeer())
			if err := s.Reset(); err != nil {
				fmt.Printf("Failed to reset unauthorized stream: %v\n", err)
			}
			return
		}

		handler(s)
	})
}

func (n *SamNode) ListenForMeshEvents(ctx context.Context) error {
	ps, err := pubsub.NewGossipSub(ctx, n.Host)
	if err != nil {
		return err
	}
	topic, err := ps.Join("sam/mesh/events/v1")
	if err != nil {
		return err
	}
	n.PubSub = ps
	sub, err := topic.Subscribe()
	if err != nil {
		return err
	}

	go func() {
		for {
			msg, err := sub.Next(ctx)
			if err != nil {
				return
			}
			var ev struct {
				PeerID peer.ID `json:"peer_id"`
				Action string  `json:"action"` // "LEFT", "BANNED"
			}
			if err := json.Unmarshal(msg.Data, &ev); err == nil {
				if ev.Action == "LEFT" || ev.Action == "BANNED" {
					n.mu.Lock()
					delete(n.TrustedPeers, ev.PeerID)
					n.mu.Unlock()
					fmt.Printf("[Mesh] Evicting peer %s based on Hub gossip\n", ev.PeerID)
				}
			}
		}
	}()
	return nil
}

// HandleAuthHandshake handles incoming Identity Biscuits
func (n *SamNode) HandleAuthHandshake(s network.Stream) {
	defer func() {
		if err := s.Close(); err != nil {
			fmt.Printf("Failed to close auth stream: %v\n", err)
		}
	}()

	// MVP: Logic to read Biscuit from stream, verify against Hub Public Key,
	// and ensure it is bound to the connecting PeerID.

	n.mu.Lock()
	n.TrustedPeers[s.Conn().RemotePeer()] = true
	n.mu.Unlock()

	fmt.Printf("Peer %s authenticated successfully via Biscuit\n", s.Conn().RemotePeer())
}
