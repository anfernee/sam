from typing import Any, Dict, List
from ..client import SamClient

def get_langchain_tools(client: SamClient, tools: List[Dict[str, Any]]) -> List[Any]:
    """Converts MCP tools into LangChain-compatible StructuredTool objects.
    
    Requires `langchain-core` to be installed.
    """
    try:
        from langchain_core.tools import StructuredTool
    except ImportError:
        raise ImportError(
            "langchain-core is required to use this adapter. "
            "Install it with `pip install langchain-core`"
        )

    lc_tools = []
    for tool in tools:
        name = tool.get("name")
        description = tool.get("description", "")
        
        # Capture the tool name in the closure
        def make_call(tool_name=name):
            async def call_remote_tool(**kwargs):
                return await client.call_tool(tool_name, kwargs)
            return call_remote_tool

        lc_tool = StructuredTool.from_function(
            name=name,
            description=description,
            coroutine=make_call(name)
        )
        lc_tools.append(lc_tool)
        
    return lc_tools
