# SAM: Sovereign Agent Mesh

SAM is a zero-trust, pure P2P networking layer for autonomous agents.

It is built for environments where centralized gateways and centralized trust are a liability. Agents discover each other over libp2p DHT, authenticate with vouches, authorize capabilities with Biscuit caveats, and communicate directly over A2A streams.

## Why SAM

- Pure P2P: no API gateway in the data path
- Zero-trust by default: every call is authenticated and authorized
- Federation isolation: separate namespaces and storage per federation
- Auditability: inspect tokens/cards and dry-run key flows

## Quick Start

### Build

```bash
make build
./bin/sam --help
```

### Run tests

```bash
make test
make test-e2e
```

### First workflow

```bash
# Authenticate
sam identity login --hub https://identity.example.com --federation finance

# Publish an agent capability
sam publish --federation finance --skill risk-audit --mcp-port 8080

# Call by capability
sam call risk-audit --federation finance --message "audit this report"

# Inspect credential artifacts
sam inspect biscuit "vendor-bot;allow_skill=risk-audit"
```

## Documentation

- Docs site: https://aojea.github.io/sam
- Manifesto: https://aojea.github.io/sam/#/README.md
- User journey (dark mesh): https://aojea.github.io/sam/#/guides/dark-mesh.md
- CLI reference: https://aojea.github.io/sam/#/cli/reference.md
- Testing guide: https://aojea.github.io/sam/#/testing.md

## Development

Key make targets:

- `make build`
- `make test`
- `make test-e2e`
- `make lint`

## License

See [LICENSE](LICENSE).
