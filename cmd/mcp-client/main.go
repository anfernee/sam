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
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	socketPath := flag.String("socket", "", "Path to Unix domain socket")
	flag.Parse()

	if *socketPath == "" {
		log.Fatal("Must specify -socket")
	}

	ctx := context.Background()

	// Override default HTTP client transport to use Unix socket
	http.DefaultClient.Transport = &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("unix", *socketPath)
		},
	}

	// Create MCP client
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "mcp-test-client",
		Version: "0.1.0",
	}, nil)

	// Connect to server using the URL (host is ignored by custom dialer)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: "http://localhost/mcp"}, nil)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			log.Printf("Failed to close session: %v", err)
		}
	}()

	// Call tool get_mesh_info
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_mesh_info",
		Arguments: map[string]any{},
	})
	if err != nil {
		log.Printf("CallTool failed: %v", err)
		return
	}

	for _, content := range result.Content {
		if textContent, ok := content.(*mcp.TextContent); ok {
			fmt.Println(textContent.Text)
		}
	}
}
