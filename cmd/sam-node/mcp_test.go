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
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMCPHandler_HTTP(t *testing.T) {
	// Setup a dummy node
	node := &SamNode{}
	handler := NewMCPHandler(node)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	client := &http.Client{}

	// Test GET on root
	req, err := http.NewRequest("GET", ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// We expect OK or MethodNotAllowed depending on exact handler implementation,
	// but it should not be 404 or 500.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status OK, MethodNotAllowed, or BadRequest on root, got %d", resp.StatusCode)
	}

	// Test GET on /mcp
	req2, err := http.NewRequest("GET", ts.URL+"/mcp", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()

	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusMethodNotAllowed && resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status OK, MethodNotAllowed, or BadRequest on /mcp, got %d", resp2.StatusCode)
	}
}
