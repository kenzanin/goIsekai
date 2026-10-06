# Spec Delta

## Purpose

Guarantees that a multi-statement write to the SQLite store takes its write lock when the transaction begins rather than at its first write, so a writer that loses the race waits and is retried by the busy timeout instead of failing partway through after work already done.

## ADDED Requirements

### Requirement: Write transactions take their lock at begin

The system SHALL open every transaction that can write against the store in immediate mode, acquiring the write lock at the moment the transaction starts. A transaction that ends up writing SHALL NOT have started under deferred locking.

#### Scenario: Transaction begin acquires the write lock
- **WHEN** a write transaction is opened
- **THEN** the write lock is held from the start of the transaction, not acquired at its first write statement

#### Scenario: Deferred transactions are not used for writes
- **WHEN** the host opens a transaction for a write path
- **THEN** that transaction is immediate; no write path opens a deferred transaction

### Requirement: Contending writers wait rather than fail

When two writers begin at the same time, one SHALL wait for the other to finish, up to the configured busy timeout, instead of receiving a busy error. A wait that exceeds the timeout SHALL still surface as an error rather than blocking indefinitely.

#### Scenario: Two writers contend
- **WHEN** two write transactions begin while another write transaction is in progress
- **THEN** the second waits for the first to complete and both succeed when the first finishes within the timeout

#### Scenario: Contention that outlasts the timeout still fails visibly
- **WHEN** a write transaction cannot acquire the lock within the configured timeout
- **THEN** it returns an error to its caller and does not wait forever

#### Scenario: Partial work is not left behind on failure
- **WHEN** a write transaction fails to acquire its lock
- **THEN** no rows from that transaction are persisted and the store is left in the state it was in before the attempt

### Requirement: Write transaction scope is unchanged

Applying immediate locking SHALL NOT alter which operations share a transaction. Operations that were previously atomic together remain atomic together, and operations that were previously separate remain separate.

#### Scenario: Multi-statement writes stay atomic
- **WHEN** an operation that performs several writes in one transaction is run under contention
- **THEN** either all of its writes are visible afterwards or none are

#### Scenario: Independent operations do not merge
- **WHEN** two independent write operations run
- **THEN** they are still separate transactions and neither observes the other's uncommitted state

### Requirement: Read-only work is unaffected

Immediate write locking SHALL NOT change how read-only queries are scheduled, and a read SHALL NOT be delayed by a writer beyond the busy timeout already in effect.

#### Scenario: Reads proceed during a write
- **WHEN** a write transaction is open and uncommitted
- **THEN** reads of unrelated rows proceed and observe the last committed state