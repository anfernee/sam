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

package discovery

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	AgentCardVersion      = "a2a.v1"
	AgentCardSignAlgo     = "libp2p-ed25519"
	AgentDHTNamespaceBase = "/sam/v1/agents/"
)

// Tool describes an advertised local tool endpoint.
type Tool struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Endpoint    string `json:"endpoint,omitempty"`
	Description string `json:"description,omitempty"`
}

// AgentCard is the signed capability document advertised in the SAM mesh.
//
// The signature covers all fields except Signature.
type AgentCard struct {
	PeerID       string    `json:"peer_id"`
	Capabilities []string  `json:"capabilities"`
	Tools        []Tool    `json:"tools,omitempty"`
	IssuedAt     time.Time `json:"issued_at"`
	Algorithm    string    `json:"alg"`
	Signature    string    `json:"signature"`
}

type agentCardPayload struct {
	PeerID       string    `json:"peer_id"`
	Capabilities []string  `json:"capabilities"`
	Tools        []Tool    `json:"tools,omitempty"`
	IssuedAt     time.Time `json:"issued_at"`
	Algorithm    string    `json:"alg"`
}

// NewAgentCard builds and signs an AgentCard for DHT advertisement.
func NewAgentCard(peerID peer.ID, capabilities []string, tools []Tool, privateKey crypto.PrivKey) (*AgentCard, error) {
	if privateKey == nil {
		return nil, fmt.Errorf("private key is required")
	}
	if peerID == "" {
		return nil, fmt.Errorf("peer ID is required")
	}
	normalizedCaps := normalizeCapabilities(capabilities)
	if len(normalizedCaps) == 0 {
		return nil, fmt.Errorf("at least one capability is required")
	}

	card := &AgentCard{
		Capabilities: normalizedCaps,
		Tools:        normalizeTools(tools),
		PeerID:       peerID.String(),
		IssuedAt:     time.Now().UTC(),
		Algorithm:    AgentCardSignAlgo,
	}

	if err := SignAgentCard(card, privateKey); err != nil {
		return nil, err
	}

	return card, nil
}

// SignAgentCard signs the card payload using the node private key.
func SignAgentCard(card *AgentCard, privateKey crypto.PrivKey) error {
	if card == nil {
		return fmt.Errorf("agent card is nil")
	}
	if privateKey == nil {
		return fmt.Errorf("private key is required")
	}
	if err := card.validateBase(); err != nil {
		return err
	}

	payload, err := card.signingPayload()
	if err != nil {
		return fmt.Errorf("encoding card payload: %w", err)
	}

	sig, err := privateKey.Sign(payload)
	if err != nil {
		return fmt.Errorf("signing card payload: %w", err)
	}
	card.Signature = base64.RawURLEncoding.EncodeToString(sig)
	return nil
}

// VerifyAgentCard verifies card integrity and signature against the embedded PeerID.
func VerifyAgentCard(card *AgentCard) error {
	if card == nil {
		return fmt.Errorf("agent card is nil")
	}
	if err := card.validateBase(); err != nil {
		return err
	}
	if strings.TrimSpace(card.Signature) == "" {
		return fmt.Errorf("card signature is required")
	}

	pid, err := peer.Decode(card.PeerID)
	if err != nil {
		return fmt.Errorf("invalid peer ID %q: %w", card.PeerID, err)
	}
	pub, err := pid.ExtractPublicKey()
	if err != nil {
		return fmt.Errorf("extracting public key from peer ID: %w", err)
	}

	payload, err := card.signingPayload()
	if err != nil {
		return fmt.Errorf("encoding card payload: %w", err)
	}

	sig, err := base64.RawURLEncoding.DecodeString(card.Signature)
	if err != nil {
		return fmt.Errorf("decoding signature: %w", err)
	}

	ok, err := pub.Verify(payload, sig)
	if err != nil {
		return fmt.Errorf("verifying signature: %w", err)
	}
	if !ok {
		return fmt.Errorf("agent card signature invalid")
	}
	return nil
}

// Sign signs the current card payload using the provided private key.
func (c *AgentCard) Sign(privateKey crypto.PrivKey) error {
	return SignAgentCard(c, privateKey)
}

// Verify checks card integrity and signature against the embedded PeerID.
func (c *AgentCard) Verify() error {
	return VerifyAgentCard(c)
}

func (c *AgentCard) validateBase() error {
	if strings.TrimSpace(c.PeerID) == "" {
		return fmt.Errorf("card peer ID is required")
	}
	if _, err := peer.Decode(c.PeerID); err != nil {
		return fmt.Errorf("invalid peer ID %q: %w", c.PeerID, err)
	}
	if len(normalizeCapabilities(c.Capabilities)) == 0 {
		return fmt.Errorf("at least one capability is required")
	}
	if c.IssuedAt.IsZero() {
		return fmt.Errorf("card issued_at is required")
	}
	if strings.TrimSpace(c.Algorithm) == "" {
		return fmt.Errorf("signature algorithm is required")
	}
	return nil
}

func (c *AgentCard) signingPayload() ([]byte, error) {
	p := agentCardPayload{
		PeerID:       c.PeerID,
		Capabilities: normalizeCapabilities(c.Capabilities),
		Tools:        normalizeTools(c.Tools),
		IssuedAt:     c.IssuedAt.UTC(),
		Algorithm:    c.Algorithm,
	}
	return json.Marshal(p)
}

// CapabilityNames returns normalized capability identifiers.
func (c *AgentCard) CapabilityNames() []string {
	if c == nil {
		return nil
	}
	return normalizeCapabilities(c.Capabilities)
}

func normalizeCapabilities(capabilities []string) []string {
	seen := make(map[string]struct{}, len(capabilities))
	out := make([]string, 0, len(capabilities))
	for _, c := range capabilities {
		n := strings.ToLower(strings.TrimSpace(c))
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func normalizeTools(tools []Tool) []Tool {
	out := make([]Tool, 0, len(tools))
	for _, tool := range tools {
		tool.Name = strings.TrimSpace(tool.Name)
		tool.Kind = strings.TrimSpace(tool.Kind)
		tool.Endpoint = strings.TrimSpace(tool.Endpoint)
		tool.Description = strings.TrimSpace(tool.Description)
		if tool.Name == "" {
			continue
		}
		out = append(out, tool)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out
}
