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

package discovery_test

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"

	protocol "sam/pkg/protocol/discovery"
	mcpprotocol "sam/pkg/protocol/mcp"
)

func TestAgentCardSignVerify(t *testing.T) {
	priv, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("GenerateEd25519Key() error = %v", err)
	}
	pid, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		t.Fatalf("IDFromPrivateKey() error = %v", err)
	}

	card, err := protocol.NewAgentCard(
		pid,
		[]string{"Inference", "search", "search"},
		priv,
		mcpprotocol.WithResources([]mcpprotocol.Resource{{Name: "local-mcp", Kind: "tool", Endpoint: "unix:///tmp/mcp.sock"}}),
	)
	if err != nil {
		t.Fatalf("NewAgentCard() error = %v", err)
	}

	if err := protocol.VerifyAgentCard(card); err != nil {
		t.Fatalf("VerifyAgentCard() error = %v", err)
	}

	capabilities := card.CapabilityNames()
	if len(capabilities) != 2 || capabilities[0] != "inference" || capabilities[1] != "search" {
		t.Fatalf("normalized capabilities = %#v, want [inference search]", capabilities)
	}
}

func TestAgentCardVerifyRejectsTamper(t *testing.T) {
	priv, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("GenerateEd25519Key() error = %v", err)
	}
	pid, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		t.Fatalf("IDFromPrivateKey() error = %v", err)
	}

	card, err := protocol.NewAgentCard(
		pid,
		[]string{"inference"},
		priv,
	)
	if err != nil {
		t.Fatalf("NewAgentCard() error = %v", err)
	}

	card.Capabilities = append(card.Capabilities, card.Capabilities[0])
	card.Capabilities[1].ID = "search"
	card.Capabilities[1].Name = "search"
	if err := protocol.VerifyAgentCard(card); err == nil {
		t.Fatal("VerifyAgentCard() should fail for tampered card")
	}
}

func TestAgentCardVerifyRejectsMissingSignature(t *testing.T) {
	priv, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("GenerateEd25519Key() error = %v", err)
	}
	pid, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		t.Fatalf("IDFromPrivateKey() error = %v", err)
	}

	card := &protocol.AgentCard{
		Version: protocol.AgentCardVersion,
		Name:    "sam-agent-" + pid.String(),
		Capabilities: []protocol.Capability{{
			ID:   "inference",
			Name: "inference",
		}},
		PeerID:    pid.String(),
		IssuedAt:  time.Now().UTC(),
		Algorithm: protocol.AgentCardSignAlgo,
	}

	if err := protocol.VerifyAgentCard(card); err == nil {
		t.Fatal("VerifyAgentCard() should fail without signature")
	}
}
