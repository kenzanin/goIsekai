
## Purpose

Makes the lane state of the bounded worker pool observable from inside the running process, so an operator diagnosing a slow page or a stuck background job can see queue depth, concurrency actually in use, and how long work is taking, without attaching an external metrics backend.

## Requirements

### Requirement: Lane state is reportable

The system SHALL report, for each background work lane, the number of jobs waiting, the number of jobs currently running, and the configured capacity for that lane. A lane that has never run a job SHALL report zero running jobs rather than being omitted.

#### Scenario: Idle lanes report their state
- **WHEN** lane state is read while no job is running and nothing is queued
- **THEN** every lane reports zero waiting and zero running

#### Scenario: Queued work is visible
- **WHEN** work is enqueued on a lane whose workers are all occupied
- **THEN** that lane reports a waiting count of at least one and a running count equal to its worker capacity

#### Scenario: Every lane is represented
- **WHEN** lane state is read
- **THEN** all four lanes are present, each with waiting, running, and capacity values, regardless of which lanes have been used

### Requirement: Job duration is recorded and summarizable

The system SHALL record the elapsed time of each completed job and SHALL be able to summarize recorded durations for a lane, including a median and a high percentile. Summarizing an empty set of durations SHALL yield a clearly absent result rather than a zero duration that reads as instantaneous work.

#### Scenario: Completed job duration is available
- **WHEN** a job on a lane completes
- **THEN** its elapsed time is recorded against that lane

#### Scenario: Percentiles reflect recorded durations
- **WHEN** durations for a lane are summarized after several jobs of differing length have completed
- **THEN** the reported median and high percentile each fall between the shortest and longest recorded duration

#### Scenario: Empty lane summary
- **WHEN** durations are summarized for a lane with no completed jobs
- **THEN** the summary reports that no durations are available rather than reporting zero

### Requirement: Lane state is readable without external dependencies

The system SHALL expose lane state through the host's own interfaces and SHALL NOT require an external metrics collector, agent, or additional service to be running.

#### Scenario: State reachable from the running host
- **WHEN** the host is running and has served at least one request
- **THEN** lane state can be read through the host without starting any additional process

#### Scenario: Reporting does not disturb running work
- **WHEN** lane state is read while jobs are running
- **THEN** those jobs complete normally and the read adds no measurable delay to them

### Requirement: Failed and cancelled work is distinguishable

A summary of lane activity SHALL distinguish work that failed from work that was abandoned before it started, so a lane that is quietly discarding cancelled work is not mistaken for a lane that is erroring.

#### Scenario: Cancellation is not reported as failure
- **WHEN** a job is cancelled before it begins running
- **THEN** it is counted as abandoned and not as a failure

#### Scenario: Failure is counted as a failure
- **WHEN** a job runs and returns an error
- **THEN** it is counted as failed and is distinguishable from an abandoned job
