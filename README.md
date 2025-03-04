# Key-Value REST API Server Documentation

This documentation describes the memory-based key-value storage REST API server for system administrators.

## Overview

The application is a simple HTTP REST API server that allows storing key-value pairs in memory during program execution. All operations are logged to standard output.

## System Requirements

- Go 1.21 or newer version
- The specified address and port must be available (by default port 8080 on all interfaces)
- GNU Make (to use the Makefile)
- UPX (optional, for binary compression)

## Version Information

The application includes version information which is injected during the build process:

- Version format: `0.1.HEX` (where HEX is the UNIX epoch time in hexadecimal format)
- Build time: Automatically captured during compilation
- Git commit: Automatically detected from the repository

You can check the version of your application with:

```bash
# Check server version
./kvapi --version

# Check client version
./kvclient --version
```

The version information is also displayed in the help text (`--help`) and during startup.

## Limitations

- Maximum key size is 255 bytes (supports Unicode characters)
- Maximum value size is 1 MB (1048576 bytes, supports Unicode characters)
- Maximum of 100 keys can be stored at once
- Data is stored only in memory and will be lost when the application is stopped
- The application does not have database persistence
- The application does not have authentication or authorization

## Installation

```bash
# Clone the repository
git clone <repo-url>
cd <repo-directory>

# Using Makefile (recommended)
make build

# Or directly with Go command
go build -o kvapi
```

## Starting and Stopping

### Starting
```bash
# Start the application using Makefile (default port: 8080, HTTP/TCP mode)
make run

# Start the application using Makefile with custom port
make LISTEN_ADDR=:3000 run

# Start the application using Makefile on specific interface and port
make LISTEN_ADDR=127.0.0.1:8080 run

# Start the application with access restricted to local network
make ALLOWED_CIDR=192.168.0.0/16 run

# Start the application with access restricted to local machine only
make ALLOWED_CIDR=127.0.0.1/32 run

# Start the application with DROP firewall simulation
make ARGS="--fw-drop --allowed-cidr=127.0.0.1/32" run

# Start the application with REJECT firewall simulation
make ARGS="--fw-reject --allowed-cidr=127.0.0.1/32" run

# Start the application in UDP mode
make ARGS="--udp" run

# Start the application in UDP mode with custom port
make ARGS="--udp --listen :4000" run

# Or use the built-in profiles:
make run-secure    # Local network only (192.168.0.0/16)
make run-local     # Localhost only (127.0.0.1/32)
make run-fw-drop   # Localhost only with DROP firewall simulation
make run-fw-reject # Localhost only with REJECT firewall simulation

# Or directly:
./kvapi --listen :8080
./kvapi --listen :3000
./kvapi --listen 127.0.0.1:8080
./kvapi --listen :8080 --allowed-cidr 192.168.0.0/16
./kvapi --listen :8080 --allowed-cidr 127.0.0.1/32
./kvapi --udp
./kvapi --udp --listen :4000
```

When starting, the application will output to the console:
```
🚀 Starting key-value API server listening on :8080

✨ === APPLIED RULES === ✨
🌐 Network rules:
  - Listen address: :8080
🔒 IP access rules:
  - All IP addresses allowed (no restrictions) ⚠️
📊 Resource limits:
  - Maximum keys: 100
  - Maximum key size: 255 bytes
  - Maximum value size: 1048576 bytes (1 MB)
✨============================✨

📡 Server is ready to accept connections! Press Ctrl+C to stop.
```
or with custom address and port:
```
🚀 Starting key-value API server listening on 127.0.0.1:3000

✨ === APPLIED RULES === ✨
🌐 Network rules:
  - Listen address: 127.0.0.1:3000
🔒 IP access rules:
  - All IP addresses allowed (no restrictions) ⚠️
📊 Resource limits:
  - Maximum keys: 100
  - Maximum key size: 255 bytes
  - Maximum value size: 1048576 bytes (1 MB)
✨============================✨

📡 Server is ready to accept connections! Press Ctrl+C to stop.
```

If started with CIDR restriction:
```
🚀 Starting key-value API server listening on :8080

✨ === APPLIED RULES === ✨
🌐 Network rules:
  - Listen address: :8080
🔒 IP access rules:
  - Restricted to CIDR: 192.168.0.0/16
  - Non-matching IPs: 403 Forbidden response
📊 Resource limits:
  - Maximum keys: 100
  - Maximum key size: 255 bytes
  - Maximum value size: 1048576 bytes (1 MB)
✨============================✨

📡 Server is ready to accept connections! Press Ctrl+C to stop.
```

### Stopping
The application can be stopped using the CTRL+C key combination or by sending the appropriate signal (SIGTERM/SIGINT).

## Command Line Parameters

| Parameter | Description | Default Value |
|-----------|-------------|---------------|
| `--listen` | Specify the address and port to listen on (format: address:port) | `:8080` |
| `--allowed-cidr` | Allowed IP address range in CIDR format (e.g., 192.168.0.0/16). If not specified, all IPs are allowed | none (all IPs allowed) |
| `--udp` | Enable UDP mode instead of HTTP/TCP mode | `false` (HTTP/TCP mode) |

## IP Restriction

The application provides the ability to restrict access to a specific IP address range (in CIDR format). If a request comes from an IP address that is not within the allowed range:

- Default mode: The request is rejected with a `403 Forbidden` status code, and appears in yellow in the console
- Two additional firewall simulation modes are available: `DROP` and `REJECT`

### Firewall Simulation Modes

The server supports two distinct firewall simulation modes that mimic real-world firewall behavior:

#### 1. DROP Mode (`--fw-drop`)

When started with the `--fw-drop` flag, the server behaves like a firewall with a DROP policy:

- Non-matching IP addresses receive no response (connection timeout)
- Client will experience waiting until connection timeout
- All dropped packets are still logged to the console for monitoring
- Log entries will include "DROPPED (fw-drop mode)" and appear in yellow
- This simulates a true firewall DROP behavior

#### 2. REJECT Mode (`--fw-reject`)

When started with the `--fw-reject` flag, the server behaves like a firewall with a REJECT policy:

- Non-matching IP addresses receive an immediate rejection response
- Client receives a clear "Connection rejected by firewall" message
- All rejected packets are logged to the console for monitoring
- Log entries will include "REJECTED (fw-reject mode)" and appear in yellow
- This simulates a firewall that actively refuses connections with ICMP or TCP RST

### Examples of CIDR ranges:
- `127.0.0.1/32` - Only localhost (exclusively local machine)
- `192.168.0.0/16` - Entire 192.168.x.x local network
- `10.0.0.0/8` - Entire 10.x.x.x private network
- `172.16.0.0/12` - Entire 172.16-31.x.x private network

## Makefile Commands

The project's Makefile supports the following commands:

| Command | Description |
|---------|-------------|
| `make` or `make all` | Run tests and build the application |
| `make build` | Build the server application |
| `make build-linux` | Build the server for Linux platform |
| `make build-client` | Build the client application |
| `make build-client-linux` | Build the client for Linux |
| `make build-all` | Build both server and client |
| `make build-all-linux` | Build both server and client for Linux |
| `make clean` | Remove generated executable files |
| `make run` | Run the server (default port `:8080`) |
| `make run-secure` | Run the server with access restricted to local network (192.168.0.0/16) |
| `make run-local` | Run the server with access restricted to local machine (127.0.0.1/32) |
| `make run-fw-drop` | Run with DROP firewall mode (silently drops non-matching IPs) |
| `make run-fw-reject` | Run with REJECT firewall mode (actively rejects non-matching IPs) |
| `make test` | Run tests |
| `make deps` | Install dependencies |
| `make install` | Install the server to the GOPATH/bin directory |
| `make install-client` | Install the client to the GOPATH/bin directory |
| `make install-all` | Install both server and client |
| `make version` | Display the current version number |
| `make VERSION=custom.version build-all` | Build applications with a custom version number |
| `make help` | Display list of Makefile commands and descriptions |

Example of using Makefile with custom parameters:
```bash
make LISTEN_ADDR=:3000 ALLOWED_CIDR=10.0.0.0/8 run      # Run server with custom settings
make VERSION=0.2.custom build-all                        # Build with custom version
make build-all                                           # Build both server and client
```

### Binary Compression with UPX

The build process automatically detects if UPX (Ultimate Packer for eXecutables) is installed on your system. If found, it will compress the binaries to reduce their size:

- No configuration needed - compression happens automatically if UPX is available
- Uses the `--fast` compression level for quick builds with good compression ratio
- The build process first strips debugging symbols with Go compiler flags (`-s -w`)
- Compressed binaries maintain full functionality while being significantly smaller
- Ideal for distribution and deployment scenarios with limited storage
- To install UPX:
  - Ubuntu/Debian: `sudo apt-get install upx-ucl`
  - CentOS/RHEL: `sudo yum install upx`
  - macOS: `brew install upx`

If UPX is not available, the build process continues normally with uncompressed binaries (still optimized for size).

## Response Format

All API responses are returned in a standardized JSON format:

```json
{
  "status": 200,
  "message": "Operation successful",
  "key": "example-key",       // Only for successful key operations
  "value": "example-value",   // Only for successful key operations
  "data": {},                 // Optional additional data
  "timestamp": "2023-06-15T14:30:15Z"
}
```

For error responses, the format is:

```json
{
  "status": 400,
  "message": "Error message describing what went wrong",
  "timestamp": "2023-06-15T14:30:15Z"
}
```

The `status` field always contains the HTTP status code of the response.

## API Endpoints

### Ping Check
- **URL:** `/api/ping`
- **Method:** `GET`
- **Response:** JSON response with status 200 and message "PONG"
- **Response Example:**
  ```json
  {
    "status": 200,
    "message": "PONG",
    "key": "ping",
    "value": "PONG",
    "timestamp": "2023-06-15T14:30:15Z"
  }
  ```
- **Example:**
  ```bash
  curl http://localhost:8080/api/ping
  ```

### Status Query
- **URL:** `/api/status`
- **Method:** `GET`
- **Response:** JSON formatted response about the number of keys and memory usage
- **Response Example:**
  ```json
  {
    "status": 200,
    "message": "Status retrieved successfully",
    "key": "status",
    "data": {
      "key_count": 5,
      "memory_usage_bytes": 2048
    },
    "timestamp": "2023-06-15T14:30:15Z"
  }
  ```
- **Example:**
  ```bash
  curl http://localhost:8080/api/status
  ```

### Get Value
- **URL:** `/api/get?k=<key>`
- **Method:** `GET`
- **URL Parameters:** `k=[string]` - The key to query
- **Success Response:** JSON response with the key and value
- **Success Response Example:**
  ```json
  {
    "status": 200,
    "message": "Key retrieved successfully",
    "key": "test",
    "value": "example value",
    "timestamp": "2023-06-15T14:30:15Z"
  }
  ```
- **Error Response:** JSON response with status 404 if the key does not exist
- **Error Response Example:**
  ```json
  {
    "status": 404,
    "message": "Key 'test' not found",
    "key": "test",
    "timestamp": "2023-06-15T14:30:15Z"
  }
  ```
- **Example:**
  ```bash
  curl "http://localhost:8080/api/get?k=test"
  ```

### Set Value
- **URL:** `/api/set?k=<key>&v=<value>`
- **Method:** `PUT` or `POST`
- **URL Parameters:**
  - `k=[string]` - The key to set (max. 255 bytes)
  - `v=[string]` - The value to store (max. 1 MB)
- **Success Response:** JSON response with status 200 and the set key and value
- **Success Response Example:**
  ```json
  {
    "status": 200,
    "message": "Key set successfully",
    "key": "test",
    "value": "example value",
    "timestamp": "2023-06-15T14:30:15Z"
  }
  ```
- **Error Responses:** JSON response with appropriate status code and error message
- **Error Response Example:**
  ```json
  {
    "status": 400,
    "message": "key exceeds maximum size of 255 bytes",
    "key": "very_long_key",
    "timestamp": "2023-06-15T14:30:15Z"
  }
  ```
- **Example:**
  ```bash
  curl -X POST "http://localhost:8080/api/set?k=test&v=value"
  ```
  or
  ```bash
  curl -X PUT "http://localhost:8080/api/set?k=test&v=value"
  ```

## Logging

All operations are logged to standard output in the following format:
```
[2023-06-15T14:30:15.123-07:00] [GET] /api/path from [192.168.1.100] - Detailed information
```

The log format includes:
- ISO 8601 formatted timestamp with millisecond precision
- HTTP method
- Request path
- Source IP address in square brackets
- Operation details

Log entries are colorized based on HTTP status codes:
- Green: Successful responses (HTTP 2xx)
- Red: Redirects and client errors (HTTP 3xx and 4xx)
- Yellow: Server errors (HTTP 5xx)

IP restriction related events appear in a special format with yellow highlighting:
```
[2023-06-15T14:30:15.123-07:00] [REJECTED] GET /api/ping from [203.0.113.5] - Access denied (IP not in allowed CIDR)
```

The server logs all requests, including requests to undefined routes (404 Not Found errors).

Examples of log messages:
```
[2023-06-15T14:30:15.123-07:00] [GET] /api/ping from [127.0.0.1] - PONG                                     # Green (200 OK)
[2023-06-15T14:30:15.456-07:00] [GET] /api/status from [192.168.1.100] - Status: 5 keys, 2048 bytes         # Green (200 OK)
[2023-06-15T14:30:16.789-07:00] [GET] /api/get from [10.0.0.5] - Retrieved key 'test' with value 'value'    # Green (200 OK)
[2023-06-15T14:30:17.123-07:00] [POST] /api/set from [10.0.0.5] - Set key 'test' to value 'value'           # Green (200 OK)
[2023-06-15T14:30:18.456-07:00] [POST] /api/set from [10.0.0.5] - Error setting key 'very_long_key': key exceeds maximum size of 255 bytes  # Red (400 Bad Request)
[2023-06-15T14:30:19.789-07:00] [REJECTED] GET /api/ping from [203.0.113.5] - Access denied (IP not in allowed CIDR)  # Yellow (Rejected)
[2023-06-15T14:30:20.123-07:00] [GET] /lskdjflksdjf from [127.0.0.1] - Route not found                      # Red (404 Not Found)
```

## Error Handling

All errors are returned as JSON responses with the appropriate HTTP status code:

- `400 Bad Request` - For client errors like missing parameters or exceeding size limits
- `403 Forbidden` - If the request IP address is not within the allowed CIDR range
- `404 Not Found` - If a non-existent key is queried
- `405 Method Not Allowed` - If an inappropriate HTTP method is used for an endpoint
- `500 Internal Server Error` - For server-side errors

## Security Considerations

- If a specific IP address is specified for the `--listen` parameter (e.g., `127.0.0.1:8080`), the server will only be accessible on that interface.
- If only the port is specified (e.g., `:8080`), the server will be accessible on all interfaces, which could pose a potential security risk.
- The `--allowed-cidr` parameter can be used to restrict which IP addresses the server accepts requests from (e.g., only from internal network).
- The server does not use encryption, so it is recommended to use it only on trusted networks.
- The size limitations help prevent excessive memory usage, but server overload is still possible.

## Monitoring

- The application's status can be monitored through standard output. Details of all requests and responses are logged.
- The `/api/status` endpoint can be used to track the number of keys and memory usage in real-time.
- Security events (rejected requests) appear highlighted in yellow on the console.

## Protocol Modes

The application can operate in two different protocol modes:

### HTTP/TCP Mode (Default)

This is the default mode where the server listens for HTTP requests over TCP. All endpoints are accessed via HTTP as documented in the API Endpoints section.

### UDP Mode

When started with the `--udp` flag, the server uses a simple UDP-based protocol instead of HTTP. In UDP mode, the application accepts text commands and returns JSON responses.

#### UDP Command Format

Commands should be sent as plain text to the configured UDP port:

| Command | Description | Example |
|---------|-------------|---------|
| `PING` | Check if the server is alive | `PING` |
| `STATUS` | Get server status | `STATUS` |
| `GET <key>` | Retrieve a value by key | `GET mykey` |
| `SET <key> <value>` | Set a key-value pair | `SET mykey myvalue` |

#### UDP Response Format

Responses are returned as JSON in the same format as the HTTP API:

```json
{
  "status": 200,
  "message": "Operation successful",
  "key": "example-key",       // Only for successful key operations
  "value": "example-value",   // Only for successful key operations
  "data": {},                 // Optional additional data
  "timestamp": "2023-06-15T14:30:15Z"
}
```

#### UDP Example Usage

Using `netcat` (nc) to communicate with the UDP server:

```bash
# Send a PING command with a 1-second timeout
echo "PING" | nc -u -w 1 localhost 8080

# Get status information with a 1-second timeout
echo "STATUS" | nc -u -w 1 localhost 8080

# Retrieve a value by key with a 1-second timeout
echo "GET mykey" | nc -u -w 1 localhost 8080

# Set a key-value pair with a 1-second timeout
echo "SET mykey myvalue" | nc -u -w 1 localhost 8080

# Set a key with a value containing spaces with a 1-second timeout
echo "SET greeting Hello, World!" | nc -u -w 1 localhost 8080
```

The `-w 1` parameter sets a 1-second timeout, so the connection will automatically close after receiving data or after 1 second, whichever comes first. Adjust the timeout value as needed for your environment.

### Go Client

The repository includes a Go client (`kvclient`) for interacting with the server in both HTTP and UDP modes. The client provides a user-friendly interface with colored output, proper error handling, and timeout management.

#### Building the Client

```bash
# Build just the client
make build-client

# Build both server and client
make build-all

# Install the client to your GOPATH/bin
make install-client

# Install both server and client to your GOPATH/bin
make install-all
```

#### Client Usage

The client supports the following commands:

```bash
# Get help and usage information
./kvclient -h

# PING command (HTTP mode by default)
./kvclient PING

# STATUS command (UDP mode)
./kvclient -protocol=udp STATUS

# GET a value (with custom host and port)
./kvclient -host=192.168.1.100 -port=3000 GET mykey

# SET a value (with custom timeout in seconds)
./kvclient -timeout=5.0 SET greeting "Hello, World!"
```

All client commands return nicely formatted and color-coded responses showing:
- Response status code and description
- Message from the server
- Key and value information (when applicable)
- Timestamp of the response

#### Client Command Line Options

| Option | Description | Default Value |
|--------|-------------|---------------|
| `-protocol` | Protocol to use (`http` or `udp`) | `http` |
| `-host` | Server hostname or IP address | `localhost` |
| `-port` | Server port number | `8080` |
| `-timeout` | Timeout in seconds for waiting for a response | `2.0` |

For example:
```bash
./kvclient -protocol=udp -host=10.0.0.5 -port=4000 -timeout=3.5 GET config
```

This command will:
1. Connect to the UDP server at 10.0.0.5 on port 4000
2. Send a GET command for the key "config"
3. Wait up to 3.5 seconds for a response
4. Format and display the response with color coding
