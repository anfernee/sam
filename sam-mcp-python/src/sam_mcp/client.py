import json
import os
from typing import Any, Dict, List, Optional
from .transport import SamTransport
from .protocol import Protocol, JsonRpcError

class SamClient:
    """High-level developer interface for SAM MCP."""
    
    def __init__(self, socket_path: Optional[str] = None):
        if socket_path is None:
            socket_path = os.environ.get("SAM_MCP_SOCKET", "/tmp/sam/mcp.sock")
        self.transport = SamTransport(socket_path)
        self._request_id = 0

    async def connect(self):
        """Connects to the SAM node and performs MCP initialization."""
        await self.transport.connect()
        await self._initialize()

    async def close(self):
        """Closes the connection."""
        await self.transport.close()

    def _next_id(self) -> int:
        self._request_id += 1
        return self._request_id

    async def _initialize(self):
        """Performs MCP handshake."""
        params = {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {"name": "sam-mcp-python", "version": "0.1.0"}
        }
        req = Protocol.create_request("initialize", params, self._next_id())
        resp_str = await self.transport.send_message(json.dumps(req))
        resp = Protocol.parse_message(resp_str)
        
        if "error" in resp:
            raise JsonRpcError(resp["error"]["code"], resp["error"]["message"], resp["error"].get("data"))
            
        # Standard MCP also expects an 'initialized' notification
        notif = Protocol.create_request("notifications/initialized")
        await self.transport.send_message(json.dumps(notif))

    async def get_tools(self) -> List[Dict[str, Any]]:
        """Returns available mesh tools."""
        req = Protocol.create_request("tools/list", {}, self._next_id())
        resp_str = await self.transport.send_message(json.dumps(req))
        resp = Protocol.parse_message(resp_str)
        
        if "error" in resp:
            raise JsonRpcError(resp["error"]["code"], resp["error"]["message"], resp["error"].get("data"))
            
        return resp.get("result", {}).get("tools", [])

    async def call_tool(self, name: str, arguments: Dict[str, Any]) -> Dict[str, Any]:
        """Executes a tool over the mesh."""
        params = {
            "name": name,
            "arguments": arguments
        }
        req = Protocol.create_request("tools/call", params, self._next_id())
        resp_str = await self.transport.send_message(json.dumps(req))
        resp = Protocol.parse_message(resp_str)
        
        if "error" in resp:
            raise JsonRpcError(resp["error"]["code"], resp["error"]["message"], resp["error"].get("data"))
            
        return resp.get("result", {})

    async def __aenter__(self):
        await self.connect()
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await self.close()
