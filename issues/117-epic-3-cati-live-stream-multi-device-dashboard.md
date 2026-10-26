# Epic 3: cati live stream & multi-device dashboard

**Status:** Open
**Category:** Epic
**Priority:** Medium

## /goal

Extend `cati cam` with multi-device capture support and live stream forwarding (`cati cam stream`), rendering a multi-camera ASCII/quadblock grid in the terminal while optionally streaming ANSI streams over HTTP/WebSockets to external clients or remote `cati` instances.

Exit clause / circuit breaker: Deliver multi-device grid dashboard and streaming backend or stop and report if CPU usage exceeds 80% on 4-device streams without GPU glyph assistance.

## Out of Scope

- Cloud camera authentication / OAuth provider integrations.

## Open Decisions

- Protocol for terminal stream over HTTP: raw ANSI chunked transfer vs JSON/WebSocket payload.

## Task Table

| Ticket | What | Sprint Group | Lead | Reviewer | Status |
|--------|------|--------------|------|----------|--------|
| TBD-1  | Multi-device capture manager & tile layout engine | SG1 | Developer | Architect | Open |
| TBD-2  | ANSI stream HTTP / WebSocket endpoint | SG1 | Developer | Reviewer | Open |
| TBD-3  | Remote `cati stream` client viewer verb | SG2 | Developer | Reviewer | Open |
| TBD-4  | Multi-cam matrix Loom TUI dashboard | SG2 | Developer | Owner | Open |
