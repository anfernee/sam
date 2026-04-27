# SAM: Sovereign Agent Mesh

SAM is a minimal libp2p-based mesh with two binaries:

- `sam-hub`: OIDC bridge + Biscuit issuer + connection gater
- `sam-node`: node identity/login flow + mesh node runtime + local MCP server

The current repository intentionally focuses on basic functionality and testability.

## Build

```bash
make build
./bin/sam-node --help
./bin/sam-hub --help
```

## Basic Usage

### 1. Run the hub

```bash
SAM_OIDC_ISSUER=https://issuer.example.com \
SAM_OIDC_ID=sam-client \
SAM_OIDC_SECRET=sam-secret \
SAM_HUB_KEY=$(openssl rand -hex 32) \
./bin/sam-hub
```

### 2. Login a node

```bash
./bin/sam-node login --hub http://localhost:8080
```

This prints a login URL. After authenticating in browser, paste the returned biscuit token back into the CLI prompt.

### 3. Run a node

```bash
./bin/sam-node run
```

Or pass a token directly:

```bash
./bin/sam-node run --token <identity-biscuit>
```

## Local MCP Server

Each `sam-node` exposes a Model Context Protocol (MCP) server over a Unix domain socket to allow local processes to discover the capabilities of the local agent and get information from the mesh.

By default, the socket listens at `<datadir>/mcp.sock`. You can change this with the `--mcp-socket` flag.

To query the mesh info via the MCP server, you can use the `mcp-client` tool provided in the repository:

```bash
./bin/mcp-client -socket /path/to/mcp.sock
```

## Testing

```bash
make test
make test-e2e
make test-e2e-container
```

`test-e2e-container` runs a containerized BATS framework that starts:

- a mock OIDC container
- a `sam-hub` container
- multiple `sam-node` containers

## Documentation

- Docs index: `docs/index.html`
- Project docs landing page: `docs/README.md`
- Quickstart: `docs/quickstart.md`
- CLI reference: `docs/cli/reference.md`
- Testing guide: `docs/testing.md`

## License

See [LICENSE](LICENSE).

## Disclaimer

This is not an officially supported Google product. This project is not eligible
for the [Google Open Source Software Vulnerability Rewards
Program](https://bughunters.google.com/open-source-security).
