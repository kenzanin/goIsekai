## Purpose

Enable clean server shutdown via SIGTERM/SIGINT with request draining, resource cleanup, and optional PID file for management scripts.

## ADDED Requirements

### Requirement: Signal-driven shutdown
The host SHALL listen for SIGTERM and SIGINT signals. On receipt, it SHALL initiate a graceful shutdown sequence: stop accepting new connections, drain in-flight requests (with configurable timeout), close DB connections, close plugin runtimes, and flush logs.

#### Scenario: SIGTERM received
- **WHEN** the server receives SIGTERM
- **THEN** it stops accepting new requests, waits up to 10s for in-flight requests to complete, then closes all resources and exits cleanly

#### Scenario: Shutdown timeout exceeded
- **WHEN** in-flight requests do not complete within the shutdown timeout
- **THEN** the server force-closes remaining connections and exits

### Requirement: PID file management
The host SHALL write its PID to a file (e.g., `goisekai.pid`) on startup. A `goisekai stop` command or equivalent SHALL read the PID file and send SIGTERM.

#### Scenario: PID file created on start
- **WHEN** the server starts successfully
- **THEN** it writes its PID to `goisekai.pid` in the data directory

#### Scenario: PID file removed on exit
- **WHEN** the server shuts down (graceful or forced)
- **THEN** it removes the PID file

#### Scenario: Stop command
- **WHEN** the user runs `goisekai stop`
- **THEN** it reads the PID file and sends SIGTERM to the running process
