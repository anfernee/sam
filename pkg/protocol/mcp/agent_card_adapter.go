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
	"sam/pkg/protocol/discovery"
)

// Resource describes an MCP-compatible tool or data source advertised by an agent.
type Resource = discovery.Tool

// SetResources stores MCP resources directly in the discovery card tool list.
func SetResources(card *discovery.AgentCard, resources []Resource) {
	if card == nil {
		return
	}
	card.Tools = append([]discovery.Tool(nil), resources...)
}

// ResourcesFromCard returns the discovery card tool list as MCP resources.
func ResourcesFromCard(card *discovery.AgentCard) []Resource {
	if card == nil {
		return nil
	}
	return append([]Resource(nil), card.Tools...)
}
