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
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/sam/api"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/spf13/cobra"
)

var (
	hubAddr     string
	listenAddrs []string
	tokenFlag   string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "sam-node",
		Short: "Sovereign Agent Mesh Node",
	}

	// LOGIN COMMAND: For Headless Environments
	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Establish sovereign identity with the Hub",
		Run: func(cmd *cobra.Command, args []string) {
			dataDir, err := GetDataDir()
			if err != nil {
				log.Fatalf("Critical: %v", err)
			}

			store, err := NewStore(dataDir)
			if err != nil {
				log.Fatalf("Critical: %v", err)
			}
			defer func() {
				if err := store.Close(); err != nil {
					log.Printf("closing store: %v", err)
				}
			}()

			hubPubKey, hubAddrs, err := api.FetchConfig(context.Background(), hubAddr)
			if err != nil {
				log.Printf("Failed to fetch hub config from %s: %v", hubAddr, err)
				fmt.Println("\n💡 Tip: If you don't have a private hub running, you can try the public community mesh:")
				fmt.Printf("   sam-node login --hub https://community.sam-mesh.dev\n\n")
				log.Fatalf("Critical: Cannot proceed without a valid hub.")
			}

			priv := getOrGenerateKey(store)
			// Temporary host to determine PeerID
			tempNode, err := NewSamNode(context.Background(), priv, hubPubKey, hubAddrs, store)
			if err != nil {
				log.Fatalf("Failed to initialize identity: %v", err)
			}
			defer func() {
				if err := tempNode.Host.Close(); err != nil {
					log.Printf("closing temporary node host: %v", err)
				}
			}()

			loginURL := fmt.Sprintf("%s/login?peer_id=%s", hubAddr, tempNode.Host.ID())

			fmt.Println("--- Sovereign Identity Login ---")
			fmt.Printf("1. Please open the following URL in your browser:\n\n   %s\n\n", loginURL)
			fmt.Println("2. Authenticate via your OIDC provider.")
			fmt.Println("3. Copy the 'Identity Biscuit' provided at the end of the flow.")
			fmt.Print("\nPaste your Identity Biscuit here: ")

			reader := bufio.NewReader(os.Stdin)
			token, err := reader.ReadString('\n')
			if err != nil {
				log.Fatalf("Failed to read input: %v", err)
			}
			token = strings.TrimSpace(token)

			if token == "" {
				log.Fatal("Error: No token provided.")
			}

			if err := store.SaveIdentity(token); err != nil {
				log.Fatalf("Failed to save identity: %v", err)
			}

			fmt.Printf("\nSuccess! Identity stored for PeerID: %s\n", tempNode.Host.ID())
		},
	}

	// RUN COMMAND: Start the Mesh
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Start the sovereign mesh node",
		Run: func(cmd *cobra.Command, args []string) {
			dataDir, _ := GetDataDir()
			store, err := NewStore(dataDir)
			if err != nil {
				log.Fatalf("Failed to open store: %v", err)
			}
			defer func() {
				if err := store.Close(); err != nil {
					log.Printf("closing store: %v", err)
				}
			}()

			token := tokenFlag
			if token == "" {
				token, err = store.LoadIdentity()
				if err != nil {
					log.Printf("Failed to load identity: %v", err)
				}
			}
			if token == "" {
				fmt.Println("No identity found. Please run 'sam-node login' or provide --token")
				return
			}

			hubPubKey, hubAddrs, err := api.FetchConfig(context.Background(), hubAddr)
			if err != nil {
				log.Printf("Failed to fetch hub config from %s: %v", hubAddr, err)
				fmt.Println("\n💡 Tip: If you don't have a private hub running, you can try the public community mesh:")
				fmt.Printf("   sam-node run --hub https://community.sam-mesh.dev\n\n")
				log.Fatalf("Critical: Cannot proceed without a valid hub.")
			}

			priv := getOrGenerateKey(store)
			node, err := NewSamNode(context.Background(), priv, hubPubKey, hubAddrs, store)
			if err != nil {
				log.Fatalf("Failed to start mesh node: %v", err)
			}

			// Ensure the auth protocol handler is always installed.
			node.Host.SetStreamHandler(AuthProtocolID, node.HandleAuthHandshake)

			fmt.Printf("SAM Node Online.\nPeerID: %s\nListening on: %v\n", node.Host.ID(), listenAddrs)

			// Block forever
			select {}
		},
	}

	// Configure Flags
	runCmd.Flags().StringVar(&tokenFlag, "token", os.Getenv("SAM_NODE_TOKEN"), "Manual Identity Biscuit (overrides store)")
	runCmd.Flags().StringSliceVar(&listenAddrs, "listen", []string{"/ip4/0.0.0.0/udp/5001/quic-v1", "/ip4/0.0.0.0/tcp/5002"}, "libp2p Listen Addrs")
	rootCmd.PersistentFlags().StringVar(&hubAddr, "hub", "http://localhost:8080", "Hub URL")

	rootCmd.AddCommand(loginCmd, runCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// getOrGenerateKey retrieves a persistent private key or creates one if it's the first run
func getOrGenerateKey(s *Store) crypto.PrivKey {
	kb, _ := s.LoadKey()
	if len(kb) == 0 {
		fmt.Println("[Store] Generating new Peer Identity...")
		priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
		if err != nil {
			log.Fatalf("Failed to generate key: %v", err)
		}
		raw, _ := crypto.MarshalPrivateKey(priv)
		if err := s.SaveKey(raw); err != nil {
			log.Fatalf("Failed to save key: %v", err)
		}
		return priv
	}
	priv, err := crypto.UnmarshalPrivateKey(kb)
	if err != nil {
		log.Fatalf("Corrupt key in store: %v", err)
	}
	return priv
}
