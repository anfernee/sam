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

package integration_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	corecrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve test file path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}

func buildBinary(t *testing.T, pkgPath string) string {
	t.Helper()
	root := repoRoot(t)
	out := filepath.Join(t.TempDir(), filepath.Base(pkgPath))
	cmd := exec.Command("go", "build", "-o", out, pkgPath)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building %s failed: %v\n%s", pkgPath, err, string(output))
	}
	return out
}

func runCommand(
	t *testing.T,
	cwd string,
	timeout time.Duration,
	env []string,
	stdin string,
	name string,
	args ...string,
) (string, string, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = cwd
	if len(env) > 0 {
		cmd.Env = append(cmd.Env, env...)
	}
	if stdin != "" {
		cmd.Stdin = bytes.NewBufferString(stdin)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return stdout.String(), stderr.String(), context.DeadlineExceeded
	}
	return stdout.String(), stderr.String(), err
}

func startMockHubConfigServer(t *testing.T) string {
	t.Helper()

	pubKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating hub public key: %v", err)
	}

	bootstrapPriv, _, err := corecrypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatalf("generating bootstrap key: %v", err)
	}
	bootstrapPeerID, err := peer.IDFromPrivateKey(bootstrapPriv)
	if err != nil {
		t.Fatalf("deriving bootstrap peer id: %v", err)
	}
	bootstrapAddr := "/ip4/127.0.0.1/tcp/4002/p2p/" + bootstrapPeerID.String()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/config":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"public_key":      hex.EncodeToString(pubKey),
				"mesh_id":         "test-mesh",
				"bootstrap_nodes": []string{bootstrapAddr},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	return ts.URL
}
