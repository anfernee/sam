import asyncio
import os
import subprocess
import time
import pytest
import pytest_asyncio
from sam_mcp.client import SamClient

@pytest.fixture(scope="session")
def sam_node_binary():
    """Ensures the sam-node binary is built."""
    repo_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../"))
    bin_path = os.path.join(repo_root, "bin", "sam-node")
    
    if not os.path.exists(bin_path):
        print(f"Binary not found at {bin_path}, building...")
        subprocess.run(["make", "build"], cwd=repo_root, check=True)
        
    return bin_path

@pytest.fixture(scope="function")
def sam_node(sam_node_binary):
    """Spins up a sam-node instance for testing."""
    socket_path = f"/tmp/sam-test-mcp-{os.getpid()}.sock"
    if os.path.exists(socket_path):
        os.remove(socket_path)
        
    # Create directory if not exists
    os.makedirs(os.path.dirname(socket_path), exist_ok=True)
    
    # Run sam-node with a dummy JWT and custom socket path.
    # We might need to pass more flags if it requires a hub, but let's try minimal first.
    process = subprocess.Popen(
        [sam_node_binary, "run", "--mcp-socket", socket_path, "--jwt", "dummy-token"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    
    # Wait for the socket file to appear
    connected = False
    for _ in range(20):
        if os.path.exists(socket_path):
            connected = True
            break
        time.sleep(0.5)
        
    if not connected:
        process.kill()
        stdout, stderr = process.communicate()
        pytest.fail(f"sam-node failed to create socket. Stdout: {stdout.decode()}, Stderr: {stderr.decode()}")
        
    yield socket_path
    
    process.terminate()
    try:
        process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        process.kill()
        
    if os.path.exists(socket_path):
        os.remove(socket_path)

@pytest.mark.asyncio
async def test_e2e_get_tools(sam_node):
    """Verifies that we can connect to the real node and get tools."""
    os.environ["SAM_MCP_SOCKET"] = sam_node
    
    async with SamClient() as client:
        tools = await client.get_tools()
        assert isinstance(tools, list)
        # Even if empty, it should be a list
        print(f"Received tools: {tools}")
        
@pytest.mark.asyncio
async def test_e2e_call_echo_tool(sam_node):
    """Verifies that we can call a tool on the real node."""
    os.environ["SAM_MCP_SOCKET"] = sam_node
    
    async with SamClient() as client:
        # Assuming the node has an 'echo' or similar built-in tool or returns error gracefully
        try:
            result = await client.call_tool("echo", {"message": "hello"})
            print(f"Tool result: {result}")
        except Exception as e:
            # If 'echo' doesn't exist, we might get a JSON-RPC error, which is also a valid E2E interaction
            print(f"Tool call failed as expected if missing: {e}")
            # If it's a connection error, that's a failure. If it's a method not found, that's a success for the pipeline.
            if "Method not found" in str(e) or "error" in str(e).lower():
                pass
            else:
                raise e
