#!/usr/bin/env bats

setup() {
  export SAM_NODE_BINARY="${SAM_NODE_BINARY:-./bin/sam-node}"
  if [[ ! -x "$SAM_NODE_BINARY" ]]; then
    skip "sam-node binary not found at $SAM_NODE_BINARY"
  fi

  export TEST_TMPDIR
  TEST_TMPDIR="$(mktemp -d)"
  export HOME="$TEST_TMPDIR/home"
  export XDG_CONFIG_HOME="$HOME/.config"
  mkdir -p "$XDG_CONFIG_HOME"

  # Start mock hub config server
  python3 -c '
import http.server
import socketserver
import sys

PORT = 8080

class Handler(http.server.SimpleHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/api/v1/config":
            self.send_response(200)
            self.send_header("Content-type", "application/json")
            self.end_headers()
            self.wfile.write(b"{\"public_key_hex\":\"0000000000000000000000000000000000000000000000000000000000000000\",\"mesh_id\":\"test-mesh\",\"bootstrap_nodes\":[\"/ip4/127.0.0.1/tcp/4002/p2p/QmYyQSo1sn1GjUuQwca9AdvV8Zeyvmxrww8dDnewPrfJs9\"]}")
        else:
            self.send_response(404)
            self.end_headers()

socketserver.TCPServer.allow_reuse_address = True
with socketserver.TCPServer(("", PORT), Handler) as httpd:
    httpd.serve_forever()
' &
  MOCK_HUB_PID=$!
  export MOCK_HUB_PID
  sleep 0.5
}

teardown() {
  kill "$MOCK_HUB_PID" || true
  wait "$MOCK_HUB_PID" 2>/dev/null || true
  rm -rf "$TEST_TMPDIR"
}

@test "sam-node run with token reaches online state" {
  log_file="$TEST_TMPDIR/run.log"
  "$SAM_NODE_BINARY" run --token test-token --listen /ip4/127.0.0.1/udp/0/quic-v1 --listen /ip4/127.0.0.1/tcp/0 >"$log_file" 2>&1 &
  pid=$!

  online=""
  for _ in {1..40}; do
    if grep -q "SAM Node Online" "$log_file"; then
      online="yes"
      break
    fi
    sleep 0.1
  done

  kill "$pid" >/dev/null 2>&1 || true
  wait "$pid" >/dev/null 2>&1 || true

  [[ "$online" == "yes" ]]
}

@test "sam-node login enables run without --token" {
  run bash -c "printf 'persisted-token\n' | '$SAM_NODE_BINARY' login"
  [[ "$status" -eq 0 ]]

  log_file="$TEST_TMPDIR/run-stored.log"
  "$SAM_NODE_BINARY" run --listen /ip4/127.0.0.1/udp/0/quic-v1 --listen /ip4/127.0.0.1/tcp/0 >"$log_file" 2>&1 &
  pid=$!

  online=""
  for _ in {1..40}; do
    if grep -q "SAM Node Online" "$log_file"; then
      online="yes"
      break
    fi
    sleep 0.1
  done

  kill "$pid" >/dev/null 2>&1 || true
  wait "$pid" >/dev/null 2>&1 || true

  [[ "$online" == "yes" ]]
}
