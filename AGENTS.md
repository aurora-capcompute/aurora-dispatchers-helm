<!-- codebase-memory-mcp:start -->
# Codebase Knowledge Graph (codebase-memory-mcp)

This project uses codebase-memory-mcp to maintain a knowledge graph of the codebase.
Always prefer MCP graph tools over grep, glob, or file search for code discovery.

Priority order:

1. `search_graph`
2. `trace_path`
3. `get_code_snippet`
4. `query_graph`
5. `get_architecture`

Fall back to text search for string literals, configuration, non-code files, or
when graph results are insufficient.
<!-- codebase-memory-mcp:end -->
