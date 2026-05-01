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

	// Test GET on root (should be 404 now)
	req, err := http.NewRequest("GET", ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status NotFound on root, got %d", resp.StatusCode)
	}

	// Test GET on /mcp/events
	req2, err := http.NewRequest("GET", ts.URL+"/mcp/events", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()

	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status OK or BadRequest on /mcp/events, got %d", resp2.StatusCode)
	}
}
