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
	"sort"
	"strings"

	"sam/pkg/protocol/discovery"
)

const cardProtocolName = "mcp"

// Resource describes an MCP-compatible tool or data source advertised by an agent.
type Resource struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Endpoint    string `json:"endpoint,omitempty"`
	Description string `json:"description,omitempty"`
}

type cardMetadata struct {
	Resources []Resource `json:"resources,omitempty"`
}

// WithResources attaches MCP-specific resource metadata to a generic discovery card.
func WithResources(resources []Resource) discovery.CardOption {
	return func(card *discovery.AgentCard) error {
		return SetResources(card, resources)
	}
}

// SetResources stores MCP resources in the card's protocol-specific payload.
func SetResources(card *discovery.AgentCard, resources []Resource) error {
	if card == nil {
		return nil
	}
	normalized := normalizeResources(resources)
	if len(normalized) == 0 {
		return nil
	}
	return card.SetProtocolPayload(cardProtocolName, cardMetadata{Resources: normalized})
}

// ResourcesFromCard extracts MCP resources from a generic discovery card.
func ResourcesFromCard(card *discovery.AgentCard) []Resource {
	if card == nil {
		return nil
	}
	var metadata cardMetadata
	if err := card.DecodeProtocolPayload(cardProtocolName, &metadata); err != nil {
		return nil
	}
	return normalizeResources(metadata.Resources)
}

func normalizeResources(resources []Resource) []Resource {
	out := make([]Resource, 0, len(resources))
	for _, resource := range resources {
		resource.Name = strings.TrimSpace(resource.Name)
		resource.Kind = strings.TrimSpace(resource.Kind)
		resource.Endpoint = strings.TrimSpace(resource.Endpoint)
		resource.Description = strings.TrimSpace(resource.Description)
		if resource.Name == "" {
			continue
		}
		out = append(out, resource)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out
}
