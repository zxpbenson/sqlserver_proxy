This is a simple program that supports only one scenario:
SQL Server Mirroring Cluster.

Some BI tools (e.g., Redash) or other applications connecting to SQL Server only allow a single hostname for the database connection.

This program listens on a local port and forwards connections to the active SQL Server instance in the mirroring setup.

It protects BI tools from disruptions caused by failovers in a SQL Server mirrored cluster.

Requirements:

1. SQLServer 2012 or later
2. Linux or Windows
3. Golang compiler 1.20+
4. Make tool

Compile:

git checkout http://github.com/zxpbenson/sqlserver_proxy.git
cd sqlserver_proxy
make
chmod +x sqlserver_proxy

Run:

usage:
sqlserver_proxy -help
sqlserver_proxy [-config nodes.json] [-inerval 10s] [-port 1433]

nodes.json template:

[
  {
    "host": "host1",
    "port": 1433,
    "user": "user",
    "password": "pass",
    "database": "databaseName"
  },
  {
    "host": "host2",
    "port": 1433,
    "user": "user",
    "password": "pass",
    "database": "databaseName"
  }
]

