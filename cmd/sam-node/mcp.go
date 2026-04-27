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
	"context"
	"fmt"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SendMessageParams defines the parameters for the send_message tool.
type SendMessageParams struct {
	PeerID  string `json:"peer_id" jsonschema:"The Peer ID of the target agent"`
	Message string `json:"message" jsonschema:"The message content"`
}

// handleSendMessage implements the send_message tool.
func handleSendMessage(ctx context.Context, req *mcp.CallToolRequest, params SendMessageParams) (*mcp.CallToolResult, any, error) {
	response := fmt.Sprintf("Simulated sending message to %s: %s", params.PeerID, params.Message)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: response},
		},
	}, nil, nil
}

// NewMCPHandler creates a new HTTP handler for the MCP server using the official SDK.
func NewMCPHandler(node *SamNode) http.Handler {
	// Create an MCP server.
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "sam-node-mcp",
		Version: "0.1.0",
	}, nil)

	// Add the send_message tool.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "send_message",
		Description: "Send a message to another agent in the mesh",
	}, handleSendMessage)

	// Add the get_mesh_info tool.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_mesh_info",
		Description: "Get information about the mesh network",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params struct{}) (*mcp.CallToolResult, any, error) {
		if node == nil {
			return nil, nil, fmt.Errorf("node not initialized")
		}
		node.mu.Lock()
		knownCount := len(node.knownPeers)
		var knownPeers []string
		for p := range node.knownPeers {
			knownPeers = append(knownPeers, p)
		}
		node.mu.Unlock()

		peers := node.Host.Network().Peers()
		dhtSize := node.DHT.RoutingTable().Size()
		
		response := fmt.Sprintf("Known peers count: %d\nKnown peers list: %v\nConnected peers: %d\nDHT Routing Table size: %d\nHub Peer ID: %s", knownCount, knownPeers, len(peers), dhtSize, node.HubPeerID)
		
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: response},
			},
		}, nil, nil
	})

	// Create the streamable HTTP handler.
	handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return server
	}, nil)

	return handler
}
