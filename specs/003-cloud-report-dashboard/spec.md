# Feature Specification: Cloud Diagnostic Report Dashboard

**Feature Branch**: `003-cloud-report-dashboard`
**Created**: 2026-06-26
**Status**: Draft
**Input**: User description: "capture the diagnostic reports in the cloud, store them and show them in a dashboard. capture device details like name, model, make, etc. Also capture all network info along with the diagnosis details. View the report can be a simple list, filtered by date, and show details when one is opened."

## Clarifications

### Session 2026-06-26

- Q: Is the dashboard a web interface (browser-based, fleet view) or an in-app Android screen? → A: Web dashboard served from the Go backend, accessible in any browser; enables multi-device fleet view.
- Q: What access protection applies to the dashboard and upload endpoint in v1? → A: No protection — fully open; auth will be added in a future iteration.
- Q: Does the dashboard show all devices combined or scoped per device? → A: All devices in one combined list with an optional device-name filter alongside the date filter.
- Q: Where in the app does the user trigger the upload? → A: Automatic — every completed diagnostic run is uploaded immediately with no user action required.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Auto-Upload Diagnostic Report to Cloud (Priority: P1)

Every time a diagnostic session completes on the Android device, the app automatically uploads the full report to the cloud with no user action. The upload captures device identity (name, model, manufacturer, OS version), all network information (interface, IPv4/IPv6 addresses, CLAT presence, DNS servers), every test result, and any 464XLAT diagnostics. The user sees an upload status indicator on the Results screen.

**Why this priority**: Automatic capture is the foundation; without it the dashboard has nothing to show. Zero friction means every run is recorded, giving complete fleet history.

**Independent Test**: Complete a diagnostic session in the app and — without any additional action — verify the report appears in the cloud backend within 10 seconds.

**Acceptance Scenarios**:

1. **Given** a diagnostic session completes, **When** the Results screen is shown, **Then** the report is automatically uploaded and a success indicator is shown to the user.
2. **Given** the device is offline when the session completes, **When** upload is attempted, **Then** the failure is shown on the Results screen and the report is queued for automatic retry when connectivity is restored.
3. **Given** a report already exists in the cloud for this session, **When** the same session is re-uploaded, **Then** the existing record is updated rather than duplicated.

---

### User Story 2 — Browse Reports in Dashboard (Priority: P2)

A user opens the dashboard and sees a chronological list of uploaded reports. Each row shows the device name, upload date/time, and a quick pass/fail summary. The list can be filtered by date range so the user can narrow to a specific time window.

**Why this priority**: Browsing is the primary consumer-facing value of cloud storage — turning a pile of stored data into a navigable history.

**Independent Test**: With at least two reports uploaded on different dates, open the dashboard, apply a date filter, and confirm only matching reports appear in the correct order.

**Acceptance Scenarios**:

1. **Given** reports exist in the cloud, **When** the dashboard is opened, **Then** reports are shown newest-first with device name, date/time, and pass/fail summary visible per row.
2. **Given** a date range is set, **When** the filter is applied, **Then** only reports whose upload timestamp falls within that range are displayed.
3. **Given** a device name filter is set, **When** applied, **Then** only reports from matching devices are shown; date and device filters can be active simultaneously.
3. **Given** no reports match the active filter, **When** the filter is applied, **Then** an empty-state message is shown instead of a blank list.
4. **Given** a large number of reports, **When** the dashboard loads, **Then** the list loads within 3 seconds and remains scrollable without freezing.

---

### User Story 3 — View Report Detail (Priority: P3)

The user taps a report in the dashboard list and sees the full detail view: device information (name, model, manufacturer, Android version), complete network snapshot (all addresses, interface names, CLAT status, DNS servers), every individual test result with status and latency, and the 464XLAT diagnostics section when present.

**Why this priority**: Detail view closes the loop — list gives orientation, detail gives actionable data for diagnosis.

**Independent Test**: Open any uploaded report's detail view and verify all four sections (device, network, tests, XLAT) are present and match the data captured at upload time.

**Acceptance Scenarios**:

1. **Given** a report in the list, **When** the user opens it, **Then** the device section shows name, model, manufacturer, and Android version.
2. **Given** the same report, **When** viewing network info, **Then** IPv4 address, IPv6 addresses, interface name, CLAT status, and DNS server list are all visible.
3. **Given** a report that includes 464XLAT diagnostics, **When** viewed, **Then** the overall XLAT chain status and all four sub-test results are shown.
4. **Given** a report detail is open, **When** the user navigates back, **Then** they return to the same scroll position in the list.

---

### Edge Cases

- What happens when the cloud service is unreachable when a session completes? The automatic upload fails silently in the background, the Results screen shows a failed indicator, and the report is queued for automatic retry when connectivity is restored.
- What happens when a report uploaded from an older app version is missing newer fields (e.g., XLAT data)? Missing fields are shown as "not available" rather than causing an error.
- What happens if the device has no name set by the user? Fall back to the device model string as the display name.
- How does the system handle very large reports (many test results, long XLAT data)? Detail view scrolls; no truncation of data.
- What happens when the date filter start date is after the end date? The filter input prevents this or shows a validation message.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The app MUST allow the user to upload a completed diagnostic session to the cloud, including device details, network info, all test results, and 464XLAT diagnostics if present.
- **FR-002**: Each uploaded report MUST capture device name, model, manufacturer, Android OS version, and a unique device identifier.
- **FR-003**: Each uploaded report MUST capture the full network snapshot: cellular interface name, IPv4 address, all IPv6 addresses, CLAT presence and interface, DNS server addresses, native IPv6 flag, and Android API level.
- **FR-004**: Upload MUST happen automatically when a diagnostic session completes — no user action is required. The Results screen MUST show an upload status indicator (uploading / success / failed).
- **FR-005**: The dashboard MUST display uploaded reports as a scrollable list, sorted newest-first, showing device name, upload date/time, and pass/fail summary per row.
- **FR-006**: The dashboard MUST support filtering the report list by a date range (start date and end date) and optionally by device name; both filters can be combined.
- **FR-007**: Tapping a report in the list MUST open a detail view showing all captured data organised into sections: Device, Network, Test Results, and 464XLAT Diagnostics.
- **FR-008**: The detail view MUST handle reports that lack a 464XLAT section gracefully, showing that section as "not available."
- **FR-009**: The system MUST prevent duplicate reports — re-uploading the same session updates the existing record.
- **FR-010**: If automatic upload fails, the Results screen MUST show an error indicator; the report MUST be queued for automatic retry when connectivity is restored. The local report is always preserved regardless of upload outcome.
- **FR-011**: The dashboard MUST be a web interface served from the backend, accessible in any browser, enabling fleet-level visibility across multiple devices without requiring the Android app.

### Key Entities

- **DiagnosticReport**: A cloud-stored record combining device details, network snapshot, test results, and optional XLAT summary. Identified by session ID.
- **DeviceInfo**: Name, model, manufacturer, OS version, unique device identifier captured at upload time.
- **NetworkSnapshot**: Full copy of network state at the time of the diagnostic run (same fields as the local NetworkInfo model).
- **ReportSummary**: Lightweight projection used in the list view — device name, upload timestamp, total tests, pass count.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can upload a completed report within 5 seconds on a stable connection.
- **SC-002**: The dashboard list loads and displays the first page of reports within 3 seconds.
- **SC-003**: 100% of data fields captured locally are preserved and retrievable from the cloud — no data loss in the round-trip.
- **SC-004**: The date filter reduces the visible list to only matching entries with no incorrect inclusions or omissions.
- **SC-005**: The detail view correctly renders all sections for 100% of reports regardless of app version that produced them.

## Assumptions

- Reports are uploaded automatically when each diagnostic session completes; no user action is required.
- A single cloud backend endpoint (hosted alongside the existing Go diagnostic server or as a separate service) stores and serves reports.
- No authentication or access control is applied in v1 — the upload endpoint and web dashboard are fully open to anyone who can reach the server. Auth is explicitly deferred to a future iteration.
- Device identifier is derived from Android device ID (non-PII hardware identifier); no personal account is required.
- The dashboard (US2, US3) is a web interface served from the Go backend, viewable in any browser; it is not an in-app Android screen.
- The existing Go server codebase is the natural home for the cloud storage backend; extending it is in scope.
- Report storage on the backend is indefinite for v1; retention policy is a future concern.
- The app already captures NetworkInfo and TestResults locally; upload reuses that data without re-running tests.
