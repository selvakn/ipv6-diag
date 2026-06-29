# Data Model: Decouple Test and Reporting Endpoints

## Android-side Model Changes

### `DiagnosticSession`

- **New field**: `testEndpointHost: String`
- **Purpose**: Stores the exact probe target host (or host:port) used during the session.
- **Validation**: Must be non-empty and match configured endpoint selection at run start.

### `CloudUploadRequest`

- **New field**: `test_endpoint`
- **Purpose**: Carries test probe endpoint context to server for dashboard visibility.
- **Rule**: Value always represents effective test endpoint for this run (custom or default).

## Android Persistence Changes (Room)

### `diagnostic_sessions` table

- **New column**: `test_endpoint_host TEXT NOT NULL DEFAULT ''`
- **Purpose**: Persist test endpoint context in local session history and support upload reconstruction.

## Server-side Model Changes

### Upload request contract

- **New property**: `test_endpoint` (string)
- **Semantics**: Endpoint used for runtime probes. Distinct from fixed reporting host.

### `reports` table

- **New column**: `test_endpoint TEXT`
- **Purpose**: Preserve probe target context for each uploaded report.
- **Migration strategy**: `ALTER TABLE` add nullable column for existing DBs.

### `ReportDetail`

- **New field**: `test_endpoint`
- **Purpose**: Expose endpoint context to dashboard detail view and API consumers.

## Entity Relationships

- One diagnostic session maps to one effective test endpoint value at run time.
- One uploaded report stores one corresponding test endpoint value.
- Fixed reporting host is global runtime configuration and not part of per-session variability.

## State/Lifecycle Notes

1. User configures optional custom test endpoint.
2. Diagnostic run captures effective endpoint (custom or default).
3. Session is persisted locally with endpoint context.
4. Upload always goes to fixed reporting host with `test_endpoint` included.
5. Dashboard reads and displays endpoint context from report detail.
