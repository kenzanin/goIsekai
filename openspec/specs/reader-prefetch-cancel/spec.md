
## Purpose

Defines when read-ahead work started by the reader is abandoned, and requires that abandonment to reach the network rather than leaving the request to finish on a lane slot nobody is waiting for.

## Requirements

### Requirement: Read-ahead work is cancellable

Read-ahead SHALL issue its requests in a form that supports cancellation. Moving away from the chapter the read-ahead belongs to SHALL abandon that read-ahead, and the abandonment SHALL reach the network layer rather than only being discarded by the caller after the response arrives.

#### Scenario: Leaving the chapter abandons its read-ahead
- **WHEN** the reader moves to a different chapter while read-ahead for the previous chapter is still in flight
- **THEN** those in-flight requests are cancelled and are not allowed to complete

#### Scenario: Abandoned requests do not hold worker capacity
- **WHEN** read-ahead for one chapter is abandoned and the reader immediately moves to another
- **THEN** the abandoned requests release their capacity without waiting for their responses

#### Scenario: Read-ahead still warms the pages it is aimed at
- **WHEN** the reader stays within a chapter
- **THEN** read-ahead completes and the pages it targeted are available to the reader without a further round trip

### Requirement: Abandonment is not surfaced as a failure

A read-ahead request abandoned because the reader moved on SHALL NOT be reported to the reader as an error, and SHALL NOT appear in host error reporting as a failed operation.

#### Scenario: Moving on does not produce a reader-visible error
- **WHEN** the reader advances a chapter while a read-ahead is in flight
- **THEN** no error surface in the reader appears and the chapter transition completes normally

#### Scenario: Abandoned read-ahead is distinguishable from a real failure
- **WHEN** read-ahead is abandoned
- **THEN** host reporting classifies it as abandoned rather than as a failed request

### Requirement: A genuine read-ahead failure still surfaces

A read-ahead request that fails for a reason other than abandonment — an unreachable host, a rejected request — SHALL still be reported, so the reader's own display of the current chapter is not silently the only signal that something is wrong upstream.

#### Scenario: Upstream failure during read-ahead is reported
- **WHEN** a read-ahead request fails because the upstream request itself failed
- **THEN** the failure is reported rather than treated as abandonment

#### Scenario: Display load is unaffected by read-ahead failure
- **WHEN** read-ahead fails for the next chapter
- **THEN** the current chapter continues to display normally and the failure does not block it
