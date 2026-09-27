# Spec Delta

## MODIFIED Requirements

### Requirement: HTMX form/action endpoints

The system SHALL handle form submissions and actions via POST endpoints. Each endpoint SHALL respond with JSON (status + optional data) instead of HTML fragments, and the client-side Alpine.js components SHALL handle response display via toast notifications. Short actions SHALL respond after completion. Long-running actions (library sync, CBZ export) SHALL enqueue their work on the worker lanes and respond immediately with a job reference and job status; the request MUST NOT stay open until the work finishes, and progress/completion SHALL be reported through the existing status surface (toast/WS).

#### Scenario: Install plugin

- **WHEN** a client sends `POST /action/install-plugin` with plugin file data
- **THEN** the server installs the plugin and returns `{"status": "ok"}` JSON

#### Scenario: Toggle plugin

- **WHEN** a client sends `POST /action/toggle-plugin/{pluginID}`
- **THEN** the server toggles the plugin state and returns `{"status": "ok", "active": true/false}` JSON

#### Scenario: Toggle library item

- **WHEN** a client sends `POST /action/toggle-library/{pluginID}/{mangaID}`
- **THEN** the server toggles the library item and returns `{"status": "ok", "in_library": true/false}` JSON

#### Scenario: Sync library

- **WHEN** a client sends `POST /action/sync`
- **THEN** the server enqueues a library-sync job and responds immediately with `{"status": "ok"}` plus a job reference, without waiting for the sync to finish; progress and completion are reported on the status surface

#### Scenario: CBZ export enqueues and returns a job reference

- **WHEN** a client sends `POST /action/export-cbz`
- **THEN** the server enqueues an export job and responds immediately with a job reference; the finished CBZ location is announced through the job status surface

#### Scenario: Duplicate sync request is not run twice

- **WHEN** a client sends `POST /action/sync` while a sync job is already queued or running
- **THEN** the response references the existing job instead of enqueuing a second sync
