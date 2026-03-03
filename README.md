# SQL Server Proxy

A lightweight TCP proxy that shields BI tools from SQL Server Mirroring Cluster failovers.

## The Problem

Many BI tools (e.g., Redash, Tableau, Power BI) and applications only allow **a single hostname** when configuring SQL Server database connections. This becomes a critical issue when using SQL Server Database Mirroring:

- During a failover, the principal role moves between nodes
- **All nodes remain network reachable**, but only the principal node can serve requests
- The mirror node will reject connections or fail to respond properly
- BI tools connecting to the wrong node will experience errors and downtime

There's no built-in way for these single-hostname tools to handle mirroring failovers gracefully.

## The Solution

This proxy acts as an intermediary that:

1. **Presents a single endpoint** to BI tools and applications
2. **Automatically detects** which node is currently the principal
3. **Routes connections** transparently to the active principal node
4. **Handles failovers seamlessly** - when the principal role moves, the proxy detects it and routes new connections to the new principal

```
┌─────────────────────────────────────────────────────────────────────────┐
│                                                                         │
│   ┌─────────────┐         ┌──────────────────┐         ┌─────────────┐ │
│   │   BI Tool   │         │                  │         │  Principal  │ │
│   │  (Redash)   │────────▶│  SQL Server      │────────▶│  Node       │ │
│   │             │  Single │  Proxy           │  Routes │  ✅ Active  │ │
│   └─────────────┘  Host   │                  │  to     └─────────────┘ │
│                          │  • Detects       │                         │
│   ┌─────────────┐         │    principal     │         ┌─────────────┐ │
│   │  Other      │         │  • Auto failover │         │  Mirror     │ │
│   │  Apps       │────────▶│  • Transparent   │────────▶│  Node       │ │
│   └─────────────┘         │    proxying      │  Skips  │  ⏸ Standby  │ │
│                          └──────────────────┘         └─────────────┘ │
│                                                                         │
│   All nodes are reachable, but only Principal can serve requests!      │
│   Proxy ensures connections always go to the right node.               │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

## Key Features

- **Single Endpoint**: BI tools only need to connect to the proxy address
- **Automatic Principal Detection**: Queries `sys.database_mirroring` to identify the principal node
- **Seamless Failover**: When principal role moves between nodes, the proxy automatically routes to the new principal
- **Transparent TCP Proxy**: Full bidirectional traffic forwarding - no protocol changes required
- **Graceful Shutdown**: Handles SIGINT/SIGTERM with configurable timeout
- **Cross-Platform**: Runs on Linux and Windows
- **Lightweight**: Minimal resource footprint

## Requirements

- SQL Server 2012 or later with Database Mirroring configured
- Linux or Windows operating system
- Go 1.20+ (for building from source)
- Make tool

## Installation

### Build from Source

```bash
git clone https://github.com/zxpbenson/sqlserver_proxy.git
cd sqlserver_proxy
make
```

The compiled binary will be located in the `build` directory.

### Install to System

```bash
make install
```

This will install the binary to `/usr/local/bin/` by default.

## Configuration

Create a `nodes.json` configuration file with your SQL Server mirroring nodes:

```json
[
  {
    "host": "hostname1",
    "port": 1433,
    "user": "monitoring_user",
    "password": "your_password",
    "database": "your_database"
  },
  {
    "host": "hostname2",
    "port": 1433,
    "user": "monitoring_user",
    "password": "your_password",
    "database": "your_database"
  }
]
```

> **Note**: The user account needs permission to query `sys.database_mirroring` view to detect the mirroring role.

## Usage

```bash
# Show help
sqlserver_proxy -help

# Run with default settings (port 1433, interval 10s, config nodes.json)
sqlserver_proxy

# Run with custom settings
sqlserver_proxy -config /path/to/nodes.json -interval 5s -port 1433
```

### Command Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `nodes.json` | Path to the nodes configuration JSON file |
| `-interval` | `10s` | Interval for checking database node status |
| `-port` | `1433` | Port for the proxy service to listen on |

## How It Works

1. **Node Detection**: The proxy periodically connects to each configured SQL Server node and queries `sys.database_mirroring` to check the `mirroring_role_desc`
2. **Role Identification**: 
   - `PRINCIPAL` → Node is marked as **enabled** (can serve requests)
   - `MIRROR` / other → Node is marked as **disabled** (cannot serve requests)
3. **Connection Routing**: When a client connects, the proxy forwards the TCP connection to the currently enabled principal node
4. **Failover Handling**: When a failover occurs, the proxy detects the role change and routes new connections to the new principal

## Typical Use Case

```
Before: BI Tool → Single Hostname → Fails when that host becomes mirror

After:  BI Tool → Proxy (single hostname) → Always connects to principal
                                    ↓
                        Automatically detects which node is principal
```

Point your BI tool to the proxy address instead of the actual SQL Server hosts. The proxy handles the rest.

## Cross Compilation

The Makefile supports cross-compilation for different platforms. Modify the `GOOS` and `GOARCH` variables in the Makefile:

```makefile
# Examples:
# Linux AMD64: GOOS=linux GOARCH=amd64
# Windows AMD64: GOOS=windows GOARCH=amd64
# Linux ARM64: GOOS=linux GOARCH=arm64
```

## Stopping the Proxy

The proxy handles graceful shutdown when receiving `SIGINT` (Ctrl+C) or `SIGTERM` signals. It will:

1. Stop accepting new connections
2. Wait for existing connections to complete (with timeout)
3. Clean up resources and exit

## License

This project is open source and available under the MIT License.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Author

[zxpbenson](https://github.com/zxpbenson)