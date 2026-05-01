import asyncio
import os
from typing import Optional

class SamTransport:
    """Handles Unix domain socket connections and HTTP-like messaging for MCP."""
    
    def __init__(self, socket_path: str):
        self.socket_path = socket_path
        self.reader: Optional[asyncio.StreamReader] = None
        self.writer: Optional[asyncio.StreamWriter] = None

    async def connect(self):
        """Establishes connection to the Unix socket."""
        self.reader, self.writer = await asyncio.open_unix_connection(self.socket_path)

    async def close(self):
        """Closes the connection."""
        if self.writer:
            self.writer.close()
            await self.writer.wait_closed()
        self.reader = None
        self.writer = None

    async def send_message(self, data: str) -> str:
        """Sends a JSON-RPC message wrapped in HTTP POST and returns the response body."""
        if not self.writer or not self.reader:
            raise RuntimeError("Not connected to SAM node")

        data_bytes = data.encode('utf-8')
        request = (
            f"POST /mcp HTTP/1.1\r\n"
            f"Host: localhost\r\n"
            f"Content-Type: application/json\r\n"
            f"Accept: application/json, text/event-stream\r\n"
            f"Content-Length: {len(data_bytes)}\r\n"
            f"\r\n"
        ).encode('utf-8') + data_bytes

        self.writer.write(request)
        await self.writer.drain()

        # Read HTTP response headers
        headers_data = bytearray()
        while True:
            line = await self.reader.readline()
            if not line:
                raise RuntimeError("Connection closed while reading headers")
            headers_data.extend(line)
            if headers_data.endswith(b"\r\n\r\n"):
                break

        headers_str = headers_data.decode('utf-8')
        lines = headers_str.split("\r\n")
        
        # Parse status line
        status_line = lines[0]
        parts = status_line.split(" ", 2)
        if len(parts) < 2:
            raise RuntimeError(f"Invalid HTTP status line: {status_line}")
        status_code = int(parts[1])
        
        # Find Content-Length
        content_length = 0
        for line in lines[1:]:
            if line.lower().startswith("content-length:"):
                content_length = int(line.split(":", 1)[1].strip())
                break

        if not (200 <= status_code < 300):
            body = ""
            if content_length > 0:
                body_bytes = await self.reader.readexactly(content_length)
                body = body_bytes.decode('utf-8')
            raise RuntimeError(f"HTTP Error {status_code}: {body}")

        if content_length == 0:
            return ""

        body = await self.reader.readexactly(content_length)
        return body.decode('utf-8')

    async def __aenter__(self):
        await self.connect()
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await self.close()
