# Image Fetch Priority Specification

## Purpose

Orders concurrent image downloads so user-visible content (the page on
screen) loads before speculative prefetches, without changing per-host
rate limiting.

## Requirements

### Requirement: Priority-based image fetch ordering

The system SHALL accept a caller-supplied priority on every image fetch:
`high` for user-visible content and `low` for speculative prefetches.
When concurrent fetches to the same host exceed that host's admission
capacity, the system SHALL admit high-priority fetches before
low-priority fetches.

#### Scenario: High-priority request jumps a queued batch

- **WHEN** a host has 10 low-priority cover fetches queued
- **AND** a high-priority fetch arrives for the same host
- **THEN** the high-priority fetch is admitted before any of the queued
  low-priority fetches that have not yet started

#### Scenario: Same priority keeps arrival order

- **WHEN** multiple fetches of equal priority are queued for a host
- **THEN** they are admitted in arrival order

### Requirement: Per-priority concurrency limits

For each host, the system SHALL admit up to 2 concurrent high-priority
image fetches and up to 1 concurrent low-priority image fetch. Fetches
beyond a lane's capacity SHALL wait without blocking the other lane.

#### Scenario: Low-priority prefetch does not block the high lane

- **WHEN** one low-priority fetch for a host is in progress
- **AND** a second low-priority fetch for that host arrives
- **THEN** the second fetch waits until the first completes
- **AND** a simultaneous high-priority fetch for that host is admitted
  without waiting for the in-progress low-priority fetch

### Requirement: Priority does not change request pacing

The system SHALL apply the same per-host pacing gap to image fetches of
every priority. A high-priority fetch SHALL NOT bypass the pacing delay
that separates consecutive requests to the same host.

#### Scenario: High priority still respects the pacing gap

- **WHEN** a high-priority fetch is admitted while a previous request to
  the same host completed less than one pacing interval ago
- **THEN** the high-priority fetch waits for the remainder of the
  pacing interval before its network request is issued

### Requirement: Priority parameter defaults to low

The image endpoint SHALL accept an optional priority parameter. When the
parameter is absent or not one of the recognized values, the fetch SHALL
be treated as low priority, so existing clients that do not send the
parameter keep working unchanged.

#### Scenario: Request without the parameter

- **WHEN** the browser requests an image without a priority parameter
- **THEN** the fetch is queued as low priority and the image is served
  normally

#### Scenario: Request with an unrecognized value

- **WHEN** the browser requests an image with an unrecognized priority
  value
- **THEN** the fetch is queued as low priority and no error is returned
