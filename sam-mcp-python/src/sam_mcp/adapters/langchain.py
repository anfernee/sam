from typing import Any, Dict, List
from ..client import SamClient

def get_langchain_tools(client: SamClient, tools: List[Dict[str, Any]]) -> List[Any]:
    """Converts MCP tools into LangChain-compatible StructuredTool objects.
    
    Requires `langchain-core` and `pydantic` to be installed.
    """
    try:
        from langchain_core.tools import StructuredTool
        from pydantic import create_model, Field
    except ImportError:
        raise ImportError(
            "langchain-core and pydantic are required to use this adapter. "
            "Install them or ensure they are available via langchain."
        )

    lc_tools = []
    for tool in tools:
        name = tool.get("name")
        description = tool.get("description", "")
        input_schema = tool.get("inputSchema", {})
        
        properties = input_schema.get("properties", {})
        required = input_schema.get("required", [])
        
        fields = {}
        for prop_name, prop_schema in properties.items():
            prop_type = prop_schema.get("type")
            prop_desc = prop_schema.get("description", "")
            
            python_type = Any
            if prop_type == "string":
                python_type = str
            elif prop_type == "integer":
                python_type = int
            elif prop_type == "number":
                python_type = float
            elif prop_type == "boolean":
                python_type = bool
            elif prop_type == "array":
                python_type = list
            elif prop_type == "object":
                python_type = dict
                
            default = ... if prop_name in required else None
            
            fields[prop_name] = (python_type, Field(default=default, description=prop_desc))
            
        args_schema = None
        if fields:
            args_schema = create_model(f"{name}Schema", **fields)

        # Capture the tool name in the closure
        def make_call(tool_name=name):
            async def call_remote_tool(**kwargs):
                return await client.call_tool(tool_name, kwargs)
            return call_remote_tool

        lc_tool = StructuredTool.from_function(
            name=name,
            description=description,
            coroutine=make_call(name),
            args_schema=args_schema
        )
        lc_tools.append(lc_tool)
        
    return lc_tools
