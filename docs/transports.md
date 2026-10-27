# Transports

`service.Service` is the use-case layer. It does not know about HTTP. `internal/httpapi` is the REST adapter: it authenticates a bearer token, calls a service method, and writes JSON.

JSON-RPC or gRPC would be another adapter that calls the same methods. `api/proto/pursuit/v1/pursuit.proto` names that future contract and is not compiled or served.

SQLite is the default store for one local daemon. Postgres is the store for replicas. Both go through `internal/sqlstore`.

There is no MCP adapter. Agents use the CLI, which calls this HTTP API.
