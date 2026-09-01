## Delta: Plugin Runtime — needs_js Init hint

Extends plugin Init metadata so a plugin can declare its site requires a JavaScript-capable engine, letting the host skip the fast path in favor of the browser engine.

## ADDED Requirements

### Requirement: needs_js plugin hint
The plugin Init response SHALL support an optional `needs_js` boolean. When true, the host SHALL route the plugin's requests through the browser engine rather than the tls-client fast path, provided an engine is configured.

#### Scenario: Plugin declares needs_js
- **WHEN** a plugin's Init response sets `needs_js: true` and an engine is configured
- **THEN** the host uses the browser engine for that plugin's requests instead of the fast path

#### Scenario: needs_js but engine off
- **WHEN** a plugin sets `needs_js: true` but the CDP engine is `off`
- **THEN** the host falls back to the fast path and may surface challenge errors as today

#### Scenario: Hint absent
- **WHEN** a plugin omits `needs_js`
- **THEN** the host treats it as `false` and uses the fast path, with challenge-triggered fallback still available
