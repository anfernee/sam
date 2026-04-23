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
	"bytes"
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

// CardOption lets protocol packages enrich generic discovery cards without
// leaking protocol-specific fields into discovery itself.
type CardOption func(*AgentCard) error

// Capability describes a normalized capability advertised by an agent.
type Capability struct {
	ID          string   `json:"id"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// Interface describes a protocol endpoint injected by a protocol-specific adapter.
type Interface struct {
	Protocol    string            `json:"protocol"`
	URL         string            `json:"url"`
	Binding     string            `json:"binding,omitempty"`
	Tenant      string            `json:"tenant,omitempty"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// AgentCard is the signed capability document advertised in the SAM mesh.
//
// The signature covers all fields except Signature.
type AgentCard struct {
	Version      string                     `json:"version"`
	Name         string                     `json:"name"`
	Description  string                     `json:"description,omitempty"`
	Capabilities []Capability               `json:"capabilities"`
	Interfaces   []Interface                `json:"interfaces,omitempty"`
	Protocols    map[string]json.RawMessage `json:"protocols,omitempty"`
	PeerID       string                     `json:"peer_id"`
	IssuedAt     time.Time                  `json:"issued_at"`
	Algorithm    string                     `json:"alg"`
	Signature    string                     `json:"signature"`
}

type agentCardPayload struct {
	Version      string                     `json:"version"`
	Name         string                     `json:"name"`
	Description  string                     `json:"description,omitempty"`
	Capabilities []Capability               `json:"capabilities"`
	Interfaces   []Interface                `json:"interfaces,omitempty"`
	Protocols    map[string]json.RawMessage `json:"protocols,omitempty"`
	PeerID       string                     `json:"peer_id"`
	IssuedAt     time.Time                  `json:"issued_at"`
	Algorithm    string                     `json:"alg"`
}

// NewAgentCard builds and signs an AgentCard for DHT advertisement.
func NewAgentCard(peerID peer.ID, capabilities []string, privateKey crypto.PrivKey, opts ...CardOption) (*AgentCard, error) {
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
	descriptors := make([]Capability, 0, len(normalizedCaps))
	for _, capability := range normalizedCaps {
		descriptors = append(descriptors, Capability{
			ID:          capability,
			Name:        capability,
			Description: "SAM capability " + capability,
			Tags:        []string{"sam"},
		})
	}

	card := &AgentCard{
		Version:      AgentCardVersion,
		Name:         "sam-agent-" + peerID.String(),
		Description:  "SAM agent " + peerID.String(),
		Capabilities: descriptors,
		PeerID:       peerID.String(),
		IssuedAt:     time.Now().UTC(),
		Algorithm:    AgentCardSignAlgo,
	}
	if err := ApplyCardOptions(card, opts...); err != nil {
		return nil, err
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
	if strings.TrimSpace(c.Version) == "" {
		return fmt.Errorf("card version is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("card name is required")
	}
	if strings.TrimSpace(c.PeerID) == "" {
		return fmt.Errorf("card peer ID is required")
	}
	if _, err := peer.Decode(c.PeerID); err != nil {
		return fmt.Errorf("invalid peer ID %q: %w", c.PeerID, err)
	}
	if len(c.Capabilities) == 0 {
		return fmt.Errorf("at least one capability is required")
	}
	if c.IssuedAt.IsZero() {
		return fmt.Errorf("card issued_at is required")
	}
	if strings.TrimSpace(c.Algorithm) == "" {
		return fmt.Errorf("signature algorithm is required")
	}
	if _, err := normalizeProtocolPayloads(c.Protocols); err != nil {
		return err
	}
	return nil
}

func (c *AgentCard) signingPayload() ([]byte, error) {
	protocols, err := normalizeProtocolPayloads(c.Protocols)
	if err != nil {
		return nil, err
	}
	p := agentCardPayload{
		Version:      strings.TrimSpace(c.Version),
		Name:         strings.TrimSpace(c.Name),
		Description:  strings.TrimSpace(c.Description),
		Capabilities: normalizeCapabilityDescriptors(c.Capabilities),
		Interfaces:   normalizeInterfaces(c.Interfaces),
		Protocols:    protocols,
		PeerID:       c.PeerID,
		IssuedAt:     c.IssuedAt.UTC(),
		Algorithm:    c.Algorithm,
	}
	return json.Marshal(p)
}

// CapabilityNames returns normalized capability identifiers derived from A2A skills.
func (c *AgentCard) CapabilityNames() []string {
	if c == nil {
		return nil
	}
	names := make([]string, 0, len(c.Capabilities))
	for _, capability := range c.Capabilities {
		name := strings.TrimSpace(capability.ID)
		if name == "" {
			name = strings.TrimSpace(capability.Name)
		}
		if name != "" {
			names = append(names, name)
		}
	}
	return normalizeCapabilities(names)
}

// SetProtocolPayload stores a protocol-specific payload in canonical JSON form.
func (c *AgentCard) SetProtocolPayload(name string, payload any) error {
	if c == nil {
		return fmt.Errorf("agent card is nil")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encoding protocol payload %q: %w", name, err)
	}
	return c.SetProtocolRawPayload(name, raw)
}

// SetProtocolRawPayload stores a raw protocol payload after compacting it.
func (c *AgentCard) SetProtocolRawPayload(name string, payload []byte) error {
	if c == nil {
		return fmt.Errorf("agent card is nil")
	}
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return fmt.Errorf("protocol name is required")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, payload); err != nil {
		return fmt.Errorf("protocol payload %q must be valid JSON: %w", name, err)
	}
	if c.Protocols == nil {
		c.Protocols = make(map[string]json.RawMessage, 1)
	}
	c.Protocols[name] = append(json.RawMessage(nil), compact.Bytes()...)
	return nil
}

// DecodeProtocolPayload decodes a protocol-specific payload into out.
func (c *AgentCard) DecodeProtocolPayload(name string, out any) error {
	if c == nil {
		return fmt.Errorf("agent card is nil")
	}
	name = strings.ToLower(strings.TrimSpace(name))
	raw := c.Protocols[name]
	if len(raw) == 0 {
		return fmt.Errorf("protocol payload %q not found", name)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decoding protocol payload %q: %w", name, err)
	}
	return nil
}

// ApplyCardOptions applies protocol-owned hooks to the card.
func ApplyCardOptions(card *AgentCard, opts ...CardOption) error {
	if card == nil {
		return fmt.Errorf("agent card is nil")
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(card); err != nil {
			return err
		}
	}
	return nil
}

// WithName overrides the generic display name in the discovery card.
func WithName(name string) CardOption {
	return func(card *AgentCard) error {
		card.Name = strings.TrimSpace(name)
		return nil
	}
}

// WithDescription overrides the generic description in the discovery card.
func WithDescription(description string) CardOption {
	return func(card *AgentCard) error {
		card.Description = strings.TrimSpace(description)
		return nil
	}
}

// WithInterface appends a generic protocol interface advertisement.
func WithInterface(iface Interface) CardOption {
	return func(card *AgentCard) error {
		card.Interfaces = append(card.Interfaces, iface)
		return nil
	}
}

// WithProtocolPayload stores protocol-specific metadata in the card.
func WithProtocolPayload(name string, payload any) CardOption {
	return func(card *AgentCard) error {
		return card.SetProtocolPayload(name, payload)
	}
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

func normalizeCapabilityDescriptors(in []Capability) []Capability {
	cloned := make([]Capability, 0, len(in))
	for _, capability := range in {
		capability.ID = strings.ToLower(strings.TrimSpace(capability.ID))
		capability.Name = strings.TrimSpace(capability.Name)
		capability.Description = strings.TrimSpace(capability.Description)
		if capability.ID == "" {
			capability.ID = strings.ToLower(strings.TrimSpace(capability.Name))
		}
		if capability.Name == "" {
			capability.Name = capability.ID
		}
		if capability.ID == "" {
			continue
		}
		for index := range capability.Tags {
			capability.Tags[index] = strings.TrimSpace(capability.Tags[index])
		}
		capability.Tags = normalizeCapabilities(capability.Tags)
		cloned = append(cloned, capability)
	}
	sort.Slice(cloned, func(i, j int) bool {
		if cloned[i].ID == cloned[j].ID {
			return cloned[i].Name < cloned[j].Name
		}
		return cloned[i].ID < cloned[j].ID
	})
	return cloned
}

func normalizeInterfaces(in []Interface) []Interface {
	cloned := make([]Interface, 0, len(in))
	for _, item := range in {
		item.Protocol = strings.ToLower(strings.TrimSpace(item.Protocol))
		item.URL = strings.TrimSpace(item.URL)
		item.Binding = strings.TrimSpace(item.Binding)
		item.Tenant = strings.TrimSpace(item.Tenant)
		item.Description = strings.TrimSpace(item.Description)
		if item.Protocol == "" || item.URL == "" {
			continue
		}
		if len(item.Metadata) == 0 {
			item.Metadata = nil
		}
		cloned = append(cloned, item)
	}
	sort.Slice(cloned, func(i, j int) bool {
		if cloned[i].Protocol == cloned[j].Protocol {
			if cloned[i].URL == cloned[j].URL {
				return cloned[i].Binding < cloned[j].Binding
			}
			return cloned[i].URL < cloned[j].URL
		}
		return cloned[i].Protocol < cloned[j].Protocol
	})
	return cloned
}

func normalizeProtocolPayloads(in map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make(map[string]json.RawMessage, len(in))
	for name, payload := range in {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" || len(payload) == 0 {
			continue
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, payload); err != nil {
			return nil, fmt.Errorf("protocol payload %q must be valid JSON: %w", name, err)
		}
		out[name] = append(json.RawMessage(nil), compact.Bytes()...)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}
