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

package mcp

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"

	"sam/pkg/protocol/discovery"
)

func TestWithResources(t *testing.T) {
	priv, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("GenerateEd25519Key() error = %v", err)
	}
	pid, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		t.Fatalf("IDFromPrivateKey() error = %v", err)
	}
	card, err := discovery.NewAgentCard(
		pid,
		[]string{"search"},
		[]Resource{{Name: "search", Kind: "tool", Endpoint: "mcp://search"}},
		priv,
	)
	if err != nil {
		t.Fatalf("NewAgentCard() error = %v", err)
	}
	resources := ResourcesFromCard(card)
	if len(resources) != 1 {
		t.Fatalf("len(resources) = %d, want 1", len(resources))
	}
	if resources[0].Endpoint != "mcp://search" {
		t.Fatalf("resource endpoint = %q, want %q", resources[0].Endpoint, "mcp://search")
	}
	if err := discovery.VerifyAgentCard(card); err != nil {
		t.Fatalf("VerifyAgentCard() error = %v", err)
	}
}
