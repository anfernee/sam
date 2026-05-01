import asyncio
import json
import pytest
from unittest.mock import AsyncMock, MagicMock, patch
from sam_mcp.protocol import Protocol, JsonRpcError
from sam_mcp.transport import SamTransport
from sam_mcp.client import SamClient
from sam_mcp.adapters.langchain import get_langchain_tools

# Protocol Tests
def test_protocol_create_request():
    req = Protocol.create_request("test_method", {"param": "value"}, 1)
    assert req == {
        "jsonrpc": "2.0",
        "method": "test_method",
        "params": {"param": "value"},
        "id": 1
    }

def test_protocol_create_response():
    resp = Protocol.create_response(1, {"result": "ok"})
    assert resp == {
        "jsonrpc": "2.0",
        "id": 1,
        "result": {"result": "ok"}
    }

def test_protocol_parse_message():
    msg = '{"jsonrpc": "2.0", "method": "test", "id": 1}'
    parsed = Protocol.parse_message(msg)
    assert parsed["method"] == "test"

def test_protocol_parse_invalid_json():
    with pytest.raises(JsonRpcError) as excinfo:
        Protocol.parse_message("{invalid}")
    assert excinfo.value.code == -32700

# Transport Tests
@pytest.mark.asyncio
async def test_transport_send_message():
    transport = SamTransport("/tmp/test.sock")
    
    mock_reader = AsyncMock()
    # Simulate HTTP response
    mock_reader.readline.side_effect = [
        b"HTTP/1.1 200 OK\r\n",
        b"Content-Length: 15\r\n",
        b"\r\n"
    ]
    mock_reader.readexactly.return_value = b'{"result":"ok"}'
    
    mock_writer = MagicMock()
    mock_writer.drain = AsyncMock()
    
    transport.reader = mock_reader
    transport.writer = mock_writer
    
    resp = await transport.send_message('{"method":"test"}')
    assert resp == '{"result":"ok"}'
    
    # Verify HTTP request was sent
    args, _ = mock_writer.write.call_args
    request_bytes = args[0]
    assert b"POST /mcp HTTP/1.1" in request_bytes
    assert b"Content-Length: 17" in request_bytes

# Client Tests
@pytest.mark.asyncio
async def test_client_get_tools():
    with patch("sam_mcp.client.sse_client") as mock_sse_client, \
         patch("sam_mcp.client.ClientSession") as MockClientSession:
         
        mock_cm = AsyncMock()
        mock_sse_client.return_value = mock_cm
        mock_cm.__aenter__.return_value = (MagicMock(), MagicMock())
        mock_cm.__aexit__ = AsyncMock()
        
        mock_session = MockClientSession.return_value
        mock_session.__aenter__ = AsyncMock(return_value=mock_session)
        mock_session.__aexit__ = AsyncMock()
        mock_session.initialize = AsyncMock()
        
        mock_tool = MagicMock()
        mock_tool.name = "test_tool"
        mock_tool.model_dump.return_value = {"name": "test_tool"}
        
        mock_resp = MagicMock()
        mock_resp.tools = [mock_tool]
        mock_session.list_tools = AsyncMock(return_value=mock_resp)
        
        async with SamClient(server_url="http://localhost:8080/sse") as client:
            tools = await client.get_tools()
            assert len(tools) == 1
            assert tools[0]["name"] == "test_tool"

# Adapter Tests
def test_langchain_adapter():
    class MockClient:
        pass
    
    client = MockClient()
    tools = [{"name": "test_tool", "description": "A test tool"}]
    
    # We need to mock langchain-core and pydantic imports if they are not installed
    with patch.dict("sys.modules", {"langchain_core.tools": MagicMock(), "pydantic": MagicMock()}):
        from langchain_core.tools import StructuredTool
        
        mock_structured_tool = MagicMock()
        StructuredTool.from_function.return_value = mock_structured_tool
        
        lc_tools = get_langchain_tools(client, tools)
        assert len(lc_tools) == 1
        assert lc_tools[0] == mock_structured_tool
