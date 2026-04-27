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
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPServer(t *testing.T) {
	handler := NewMCPHandler(nil)

	t.Run("initialize", func(t *testing.T) {
		reqBody := []byte(`{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0"}},"id":1}`)
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
	})

	t.Run("tools/list", func(t *testing.T) {
		reqBody := []byte(`{"jsonrpc":"2.0","method":"tools/list","id":2}`)
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}

		body := w.Body.String()
		// We expect an error because we haven't established a full session (SSE handshake + initialize).
		if !strings.Contains(body, "invalid during session initialization") {
			t.Errorf("Expected session initialization error, got: %s", body)
		}
	})
}
