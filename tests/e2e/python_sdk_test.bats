#!/usr/bin/env bats

load "lib/container_mesh.bash"

setup() {
  if ! mesh_require_docker; then
    skip "docker not available or daemon not running"
  fi

  if [[ ! -x "./bin/sam-node" || ! -x "./bin/sam-hub" ]]; then
    skip "missing binaries; run: make build"
  fi

  mesh_setup_env
}

teardown() {
  mesh_cleanup_env
}

@test "Python SDK: Connect, get tools, and call tool" {
  run mesh_start_mock_oidc
  [[ "$status" -eq 0 ]]

  run mesh_start_hub
  [[ "$status" -eq 0 ]]

  run mesh_start_node 1 "--discovery-interval 100ms --log-level debug"
  [[ "$status" -eq 0 ]]
  
  local node1_name="${MESH_PREFIX}-node-1"
  mesh_wait_for_log "${node1_name}" "SAM Node Online" 20
  mesh_wait_for_mcp_ready 1 20

  # Use the Python SDK to interact with the node
  run docker run --rm \
    --network "${MESH_NETWORK}" \
    -v "$(pwd)/sam-mcp-python:/sam-mcp-python" \
    -e PYTHONPATH=/sam-mcp-python/src \
    python:3.12 \
    bash -c 'pip install mcp >/dev/null && python3 -c "
import asyncio
from sam_mcp.client import SamClient
import os
import sys

async def main():
    os.environ[\"SAM_MCP_URL\"] = \"http://sam-node-1:8080/\"
    try:
        async with SamClient() as client:
            # Test get_tools
            tools = await client.get_tools()
            print(f\"TOOLS_COUNT:{len(tools)}\")
            
            # Test call_tool (get_mesh_info is a standard tool in sam-node)
            result = await client.call_tool(\"get_mesh_info\", {})
            print(f\"CALL_RESULT:{result}\")
            
            sys.exit(0)
    except BaseException as e:
        import traceback
        print(f\"ERROR:{e}\")
        traceback.print_exc()
        sys.exit(1)

asyncio.run(main())
"'
  echo "Python SDK output: $output"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"TOOLS_COUNT:"* ]]
  [[ "$output" == *"CALL_RESULT:"* ]]
}
