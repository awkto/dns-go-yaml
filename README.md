# DNS Server in Go with YAML/CSV Configuration

This is a simple DNS server implemented in Go, capable of loading zone data from YAML or CSV files, forwarding queries to an upstream DNS server, query logging, and caching.

## Features

-   **Zone Data**: Load DNS records from YAML or CSV files.
-   **Forwarding**: Forward DNS queries to an upstream DNS server.
-   **Query Logging**: Log DNS queries to a file.
-   **Caching**: Cache DNS responses to improve performance.
-   **Configuration**: Configure the server using a `settings.conf` file.

## Configuration

The server is configured using a `settings.conf` file. Here's an example:

```properties
# Configuration for the DNS server

# File that holds the zone data
zone_file = zone_data.csv

# Format of the zone file (yaml/csv)
zone_file_format = csv

# Port to listen on
port = 5053

# Upstream DNS server for forwarding
forwarder = 1.1.1.1

# Enable or disable forwarding (true/false)
enable_forwarding = true

# Enable query logging (true/false)
query_logging = true

# Query log file path
query_log_file = query.log
```

### Settings

-   `zone_file`: Path to the zone data file (YAML or CSV).
-   `zone_file_format`: Format of the zone data file (`yaml` or `csv`).
-   `port`: Port number to listen on.
-   `forwarder`: IP address of the upstream DNS server for forwarding queries.
-   `enable_forwarding`: Enable or disable forwarding.
-   `query_logging`: Enable or disable query logging.
-   `query_log_file`: Path to the query log file.

## Zone Data Format

### YAML

Example `zone_data.yaml`:

```yaml
records:
  - name: example.com
    type: A
    ttl: 600
    data: 192.0.2.1
  - name: www.example.com
    type: CNAME
    ttl: 600
    data: example.com
  - name: site1.example.com
    type: A
    ttl: 300
    data: 10.0.0.5
  - name: site2.example.com
    type: A
    ttl: 300
    data: 12.34.56.78
```

### CSV

Example `zone_data.csv`:

```csv
name,type,ttl,data
example.com,A,600,192.0.2.1
www.example.com,CNAME,600,example.com
site1.example.com,A,300,10.0.0.5
site2.example.com,A,300,12.34.56.78
```

## Usage

1.  Clone the repository:

    ```bash
    git clone <repository_url>
    cd dns-go-yaml
    ```

2.  Configure the server by editing the `settings.conf` file.

3.  Run the server:

    ```bash
    go run dns-server.go
    ```

## Dependencies

-   [miekg/dns](https://github.com/miekg/dns)
-   [gopkg.in/ini.v1](https://github.com/go-ini/ini)
-   [gopkg.in/yaml.v2](https://github.com/go-yaml/yaml)

## Query Logging

When query logging is enabled, the server logs all DNS queries to the specified log file.

## Caching

The server caches DNS responses to improve performance. The TTL for each record is respected, and the cache is automatically updated when records expire.
