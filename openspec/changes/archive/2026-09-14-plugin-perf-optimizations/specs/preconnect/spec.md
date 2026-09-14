## Purpose

Proactively open HTTP connections to plugin target sites at startup so the first user request skips TCP+TLS handshake overhead and responds faster.

## ADDED Requirements

### Requirement: Preconnect on plugin load
The system SHALL open HTTP connections to a plugin's primary target host when the plugin is first loaded. Preconnected connections SHALL be added to the connection pool for immediate use by subsequent plugin requests.

#### Scenario: Plugin load triggers preconnect
- **WHEN** a plugin is loaded for the first time
- **THEN** the system opens an HTTP connection to the plugin's primary host and adds it to the connection pool

#### Scenario: Preconnect fails gracefully
- **WHEN** a preconnect attempt fails (DNS error, timeout, refused)
- **THEN** the system logs a warning and continues without the preconnected connection

### Requirement: Preconnect to known hosts
The system SHALL maintain a configurable list of hosts to preconnect to at startup, independent of plugin loading. This list SHALL include hosts from recently-used plugins.

#### Scenario: Startup preconnect to known hosts
- **WHEN** the application starts
- **THEN** the system opens connections to all hosts in the known-hosts list

#### Scenario: Known hosts list updated on plugin use
- **WHEN** a plugin makes a request to a new host
- **THEN** the system adds that host to the known-hosts list for future startups

### Requirement: Preconnect concurrency limit
The system SHALL limit concurrent preconnect attempts to avoid overwhelming target sites. The default limit SHALL be 4 concurrent preconnections.

#### Scenario: Preconnect respects concurrency limit
- **WHEN** more hosts need preconnection than the concurrency limit
- **THEN** the system queues excess preconnections and processes them as slots become available

#### Scenario: Preconnect completes within timeout
- **WHEN** a preconnect attempt does not complete within 5 seconds
- **THEN** the system abandons the attempt and closes the connection
