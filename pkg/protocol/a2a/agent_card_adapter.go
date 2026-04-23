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

package a2a

import (
	"fmt"
	"sort"
	"strings"
	"time"

	mcpprotocol "sam/pkg/protocol/mcp"

	a2asdk "github.com/a2aproject/a2a-go/v2/a2a"

	"sam/pkg/protocol/discovery"
)

const protocolPayloadName = "a2a"

// ToA2ACard converts a generic discovery AgentCard into the official A2A SDK representation.
func ToA2ACard(card *discovery.AgentCard, interfaceURL string) (*a2asdk.AgentCard, error) {
	if card == nil {
		return nil, fmt.Errorf("agent card is nil")
	}
	if strings.TrimSpace(card.Version) == "" {
		return nil, fmt.Errorf("card version is required")
	}
	if strings.TrimSpace(card.Name) == "" {
		return nil, fmt.Errorf("card name is required")
	}
	if strings.TrimSpace(card.PeerID) == "" {
		return nil, fmt.Errorf("card peer ID is required")
	}
	if len(card.Capabilities) == 0 {
		return nil, fmt.Errorf("at least one capability is required")
	}

	out, _ := synthesizeA2ACard(card)
	if err := card.DecodeProtocolPayload(protocolPayloadName, &out); err == nil {
		out = normalizeA2ASDKCard(out)
	}
	if strings.TrimSpace(interfaceURL) != "" {
		out.SupportedInterfaces = []*a2asdk.AgentInterface{
			a2asdk.NewAgentInterface(strings.TrimSpace(interfaceURL), a2asdk.TransportProtocolJSONRPC),
		}
	}
	return &out, nil
}

// AgentCardFromA2A converts an A2A SDK AgentCard into the generic discovery AgentCard.
func AgentCardFromA2A(peerID string, card *a2asdk.AgentCard, resources []mcpprotocol.Resource) (*discovery.AgentCard, error) {
	if card == nil {
		return nil, fmt.Errorf("a2a agent card is nil")
	}
	peerID = strings.TrimSpace(peerID)
	if peerID == "" {
		return nil, fmt.Errorf("peer ID is required")
	}

	capabilities := make([]string, 0, len(card.Skills))
	for _, skill := range card.Skills {
		if id := strings.TrimSpace(skill.ID); id != "" {
			capabilities = append(capabilities, id)
			continue
		}
		if name := strings.TrimSpace(skill.Name); name != "" {
			capabilities = append(capabilities, name)
		}
	}
	if len(capabilities) == 0 {
		return nil, fmt.Errorf("a2a card must contain at least one skill")
	}

	out := &discovery.AgentCard{
		Version:      discovery.AgentCardVersion,
		Name:         strings.TrimSpace(card.Name),
		Description:  strings.TrimSpace(card.Description),
		Capabilities: capabilitiesFromSkills(card.Skills),
		Interfaces:   interfacesFromA2A(card.SupportedInterfaces),
		PeerID:       peerID,
		IssuedAt:     time.Now().UTC(),
		Algorithm:    discovery.AgentCardSignAlgo,
	}
	if err := mcpprotocol.SetResources(out, resources); err != nil {
		return nil, err
	}
	if err := out.SetProtocolPayload(protocolPayloadName, normalizeA2ASDKCard(*card)); err != nil {
		return nil, err
	}
	return out, nil
}

func synthesizeA2ACard(card *discovery.AgentCard) (a2asdk.AgentCard, error) {
	if card == nil {
		return a2asdk.AgentCard{}, fmt.Errorf("agent card is nil")
	}
	out := a2asdk.AgentCard{
		Capabilities: a2asdk.AgentCapabilities{Streaming: true},
		Description:  strings.TrimSpace(card.Description),
		Name:         strings.TrimSpace(card.Name),
		Skills:       skillsFromCapabilities(card.Capabilities),
		Version:      discovery.AgentCardVersion,
	}
	for _, item := range card.Interfaces {
		if item.Protocol != protocolPayloadName {
			continue
		}
		binding := a2asdk.TransportProtocolJSONRPC
		if strings.TrimSpace(item.Binding) != "" {
			binding = a2asdk.TransportProtocol(strings.TrimSpace(item.Binding))
		}
		iface := a2asdk.NewAgentInterface(strings.TrimSpace(item.URL), binding)
		iface.Tenant = strings.TrimSpace(item.Tenant)
		out.SupportedInterfaces = append(out.SupportedInterfaces, iface)
	}
	return normalizeA2ASDKCard(out), nil
}

func skillsFromCapabilities(in []discovery.Capability) []a2asdk.AgentSkill {
	out := make([]a2asdk.AgentSkill, 0, len(in))
	for _, capability := range in {
		identifier := strings.TrimSpace(capability.ID)
		if identifier == "" {
			identifier = strings.TrimSpace(capability.Name)
		}
		if identifier == "" {
			continue
		}
		out = append(out, a2asdk.AgentSkill{
			ID:          identifier,
			Name:        strings.TrimSpace(capability.Name),
			Description: strings.TrimSpace(capability.Description),
			Tags:        append([]string(nil), capability.Tags...),
		})
	}
	return normalizeSkills(out)
}

func capabilitiesFromSkills(in []a2asdk.AgentSkill) []discovery.Capability {
	out := make([]discovery.Capability, 0, len(in))
	for _, skill := range in {
		identifier := strings.TrimSpace(skill.ID)
		name := strings.TrimSpace(skill.Name)
		if identifier == "" {
			identifier = name
		}
		if name == "" {
			name = identifier
		}
		if identifier == "" {
			continue
		}
		out = append(out, discovery.Capability{
			ID:          identifier,
			Name:        name,
			Description: strings.TrimSpace(skill.Description),
			Tags:        append([]string(nil), skill.Tags...),
		})
	}
	return out
}

func interfacesFromA2A(in []*a2asdk.AgentInterface) []discovery.Interface {
	out := make([]discovery.Interface, 0, len(in))
	for _, item := range in {
		if item == nil {
			continue
		}
		out = append(out, discovery.Interface{
			Protocol: protocolPayloadName,
			URL:      strings.TrimSpace(item.URL),
			Binding:  string(item.ProtocolBinding),
			Tenant:   strings.TrimSpace(item.Tenant),
		})
	}
	return out
}

func normalizeA2ASDKCard(card a2asdk.AgentCard) a2asdk.AgentCard {
	card.Skills = normalizeSkills(card.Skills)
	card.SupportedInterfaces = normalizeInterfaces(card.SupportedInterfaces)
	card.Signatures = nil
	return card
}

func normalizeSkills(skills []a2asdk.AgentSkill) []a2asdk.AgentSkill {
	cloned := append([]a2asdk.AgentSkill(nil), skills...)
	for index := range cloned {
		cloned[index].ID = strings.TrimSpace(cloned[index].ID)
		cloned[index].Name = strings.TrimSpace(cloned[index].Name)
		cloned[index].Description = strings.TrimSpace(cloned[index].Description)
		sort.Strings(cloned[index].Tags)
		sort.Strings(cloned[index].Examples)
		sort.Strings(cloned[index].InputModes)
		sort.Strings(cloned[index].OutputModes)
	}
	sort.Slice(cloned, func(i, j int) bool {
		if cloned[i].ID == cloned[j].ID {
			return cloned[i].Name < cloned[j].Name
		}
		return cloned[i].ID < cloned[j].ID
	})
	return cloned
}

func normalizeInterfaces(in []*a2asdk.AgentInterface) []*a2asdk.AgentInterface {
	cloned := make([]*a2asdk.AgentInterface, 0, len(in))
	for _, item := range in {
		if item == nil {
			continue
		}
		copyItem := *item
		copyItem.URL = strings.TrimSpace(copyItem.URL)
		copyItem.Tenant = strings.TrimSpace(copyItem.Tenant)
		cloned = append(cloned, &copyItem)
	}
	sort.Slice(cloned, func(i, j int) bool {
		if cloned[i].URL == cloned[j].URL {
			return cloned[i].Tenant < cloned[j].Tenant
		}
		return cloned[i].URL < cloned[j].URL
	})
	return cloned
}
