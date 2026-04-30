package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/google/sam/api"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"google.golang.org/protobuf/proto"
)

func main() {
	keyHex := flag.String("key", "", "Hub private key (hex)")
	peerToBan := flag.String("peer", "", "Peer ID to ban")
	var addrs []string
	flag.Func("addr", "Multiaddresses to connect to", func(s string) error {
		addrs = append(addrs, s)
		return nil
	})
	flag.Parse()

	if *keyHex == "" || *peerToBan == "" || len(addrs) == 0 {
		log.Fatal("Missing required flags")
	}

	keyBytes, err := hex.DecodeString(*keyHex)
	if err != nil {
		log.Fatal(err)
	}
	privKey := ed25519.NewKeyFromSeed(keyBytes)

	ctx := context.Background()

	h, err := libp2p.New()
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = h.Close() }()

	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		log.Fatal(err)
	}

	connected := false
	for _, addrStr := range addrs {
		addr, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			log.Printf("Failed to parse addr %s: %v", addrStr, err)
			continue
		}
		addrInfo, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			log.Printf("Failed to get addr info for %s: %v", addrStr, err)
			continue
		}
		if err := h.Connect(ctx, *addrInfo); err != nil {
			log.Printf("Failed to connect to %s: %v", addrStr, err)
			continue
		}
		fmt.Printf("Connected to %s\n", addrStr)
		connected = true
	}
	if !connected {
		log.Fatal("Failed to connect to any peer")
	}

	topic, err := ps.Join(api.GossipEvents)
	if err != nil {
		log.Fatal(err)
	}

	event := &api.MeshEvent{
		Type:      api.MeshEvent_BANNED,
		PeerId:    *peerToBan,
		Timestamp: time.Now().Unix(),
	}

	// Sign event
	event.Signature = nil
	data, err := proto.Marshal(event)
	if err != nil {
		log.Fatal(err)
	}
	event.Signature = ed25519.Sign(privKey, data)

	eventData, err := proto.Marshal(event)
	if err != nil {
		log.Fatal(err)
	}

	// Wait for pubsub to discover peers
	fmt.Println("Waiting for pubsub peers...")
	for i := 0; i < 60; i++ {
		peers := topic.ListPeers()
		if len(peers) > 0 {
			fmt.Printf("Found %d pubsub peers: %v\n", len(peers), peers)
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if len(topic.ListPeers()) == 0 {
		fmt.Println("Warning: No pubsub peers found, publishing anyway...")
	}

	if err := topic.Publish(ctx, eventData); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Published ban event")
}
