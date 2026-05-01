import json
from typing import Any, Dict, Optional, Union

class JsonRpcError(Exception):
    def __init__(self, code: int, message: str, data: Optional[Any] = None):
        self.code = code
        self.message = message
        self.data = data
        super().__init__(f"JSON-RPC Error {code}: {message}")

    def to_dict(self) -> Dict[str, Any]:
        res = {"code": self.code, "message": self.message}
        if self.data is not None:
            res["data"] = self.data
        return res

class Protocol:
    @staticmethod
    def create_request(method: str, params: Optional[Dict[str, Any]] = None, request_id: Optional[Union[int, str]] = None) -> Dict[str, Any]:
        req = {
            "jsonrpc": "2.0",
            "method": method,
        }
        if params is not None:
            req["params"] = params
        if request_id is not None:
            req["id"] = request_id
        return req

    @staticmethod
    def create_response(request_id: Union[int, str], result: Any) -> Dict[str, Any]:
        return {
            "jsonrpc": "2.0",
            "id": request_id,
            "result": result
        }

    @staticmethod
    def create_error_response(request_id: Optional[Union[int, str]], code: int, message: str, data: Optional[Any] = None) -> Dict[str, Any]:
        error = {"code": code, "message": message}
        if data is not None:
            error["data"] = data
        return {
            "jsonrpc": "2.0",
            "id": request_id,
            "error": error
        }

    @staticmethod
    def parse_message(message_str: str) -> Dict[str, Any]:
        try:
            data = json.loads(message_str)
        except json.JSONDecodeError as e:
            raise JsonRpcError(-32700, f"Parse error: {e}. Message: {message_str}")
        
        if not isinstance(data, dict):
            raise JsonRpcError(-32600, "Invalid Request", "Message must be a JSON object")
        
        if data.get("jsonrpc") != "2.0":
            raise JsonRpcError(-32600, "Invalid Request", "Missing or invalid jsonrpc version")
            
        return data
