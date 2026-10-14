# Spec Delta

## Purpose

Defines how goIsekai schedules work: separate bounded worker lanes per workload class (UI/interactive, manga-metadata fetch, image fetch, maintenance) so bulk work never starves interactive work.

## ADDED Requirements

### Requirement: Separate worker lanes per workload class

The system SHALL run scheduled work on four independent worker lanes: an interactive lane for user-facing request orchestration (search, detail, chapter list, reader data), a fetch lane for manga metadata jobs (library sync, migration search, enrichment), an image lane for cover/page/export image fetches, and a maintenance lane for periodic upkeep (orphan pruning, FTS heal, backups, cache prune). Each lane SHALL have its own bounded queue and worker count; a full queue on any lane SHALL NOT block the interactive lane.

#### Scenario: Bulk work cannot starve the UI

- **WHEN** the fetch and image lanes are saturated with background jobs
- **THEN** new interactive jobs are still admitted and completed without waiting for those lanes to drain

#### Scenario: Maintenance stays isolated

- **WHEN** a maintenance tick runs FTS heal or cache pruning
- **THEN** the work runs on the maintenance lane serially and does not occupy interactive, fetch, or image workers

### Requirement: Enqueue-and-await on worker lanes

Request handlers SHALL enqueue typed jobs onto the appropriate lane and await a context-aware future; per-host pacing waits and lane-admission waits SHALL be paid by worker goroutines, not by HTTP request goroutines. Awaiting SHALL honour the request context: when the client goes away, the future is cancelled and the job is abandoned if it has not started.

#### Scenario: Image pacing is paid by the worker

- **WHEN** a burst of image requests arrives for one host and the pacing gap applies
- **THEN** the delays are incurred on image-lane workers while request goroutines only await their futures

#### Scenario: Client disconnect cancels the wait

- **WHEN** the request context is cancelled while a job is queued
- **THEN** the job is cancelled and no result is delivered

### Requirement: Bounded queues with explicit backpressure

Every lane queue SHALL be bounded. When the interactive queue is full, the system SHALL block admission until the request deadline and then fail with an HTTP 503. When a background queue is full, enqueue SHALL block up to a short timeout and then surface a busy indication to the caller; the system SHALL NOT spawn unbounded goroutines to absorb load.

#### Scenario: Interactive queue exhaustion

- **WHEN** the interactive queue is full and capacity is not freed before the request deadline
- **THEN** the request fails with 503 and no silent goroutine growth occurs

#### Scenario: Background queue exhaustion

- **WHEN** a background enqueue times out on a full queue
- **THEN** the caller receives a busy indication instead of the job being silently dropped or run inline

### Requirement: Job lifecycle with bounded retries

Every job SHALL move through queued → running → done, or failed after bounded retries with a final dead state. Failures caused by transient upstream errors SHALL be retried with a limit; permanent failures SHALL NOT be retried. Job state SHALL be observable through the existing status surface (toast/WS/logs).

#### Scenario: Transient failure retries then succeeds

- **WHEN** a fetch job fails once with a transient upstream error and the retry succeeds
- **THEN** the job completes as done and the result is delivered to the waiter

#### Scenario: Exhausted retries end the job

- **WHEN** a job keeps failing through its retry budget
- **THEN** the job ends in the dead state and the failure is reported on the status surface

### Requirement: Image priority and host admission preserved on the image lane

The image lane SHALL preserve the per-host admission and pacing semantics of image-fetch-priority: high-priority fetches admitted before low-priority ones, up to 2 concurrent high and 1 concurrent low per host, with the pacing gap applied between consecutive fetches to the same host. Those waits SHALL occur on image-lane workers. Identical image fetches SHALL be deduplicated at enqueue so concurrent requests for one URL share a single job.

#### Scenario: Priority ordering under saturation

- **WHEN** high-priority and low-priority image jobs contend for one host
- **THEN** high-priority jobs are admitted first while the per-host concurrency limits hold

#### Scenario: Concurrent identical URLs share one job

- **WHEN** multiple requests enqueue the same image URL before the first completes
- **THEN** one job runs and all waiters receive its result

### Requirement: Per-plugin fairness on the fetch lane

The fetch lane SHALL keep at most one in-flight job per plugin so a slow or failing source cannot monopolise fetch-lane workers.

#### Scenario: Slow plugin does not monopolise the lane

- **WHEN** many fetch jobs for a slow plugin are queued alongside jobs for other plugins
- **THEN** jobs for other plugins proceed while only one job for the slow plugin runs at a time

### Requirement: Configurable lane sizing

The system SHALL read lane worker counts and queue bounds from a `[workers]` config section (`interactive`, `fetch`, `image`, and queue sizes), applying sensible defaults when the section is absent. Values SHALL be honoured at startup and on safe config reload.

#### Scenario: Defaults without config

- **WHEN** the config file has no `[workers]` section
- **THEN** the lanes start with their default worker counts and queue bounds

#### Scenario: Reload applies new sizes

- **WHEN** a safe config reload changes `[workers]` values
- **THEN** subsequent lane sizing uses the new values without restarting the process
