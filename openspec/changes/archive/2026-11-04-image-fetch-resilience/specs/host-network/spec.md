# Spec Delta

## ADDED Requirements

### Requirement: Plugin-declared User-Agent ladder
A plugin MAY declare an ordered `user_agents` list in its `plugin.json`. The host SHALL send the first entry as the `User-Agent` of every request that plugin makes, including image fetches the host makes on its behalf, until a block response is observed. The literal entry `default` SHALL stand for the host's configured User-Agent. Empty entries SHALL be ignored. A plugin that declares no list SHALL keep the host default.

#### Scenario: First entry is used from the first request
- **WHEN** a plugin declares `user_agents` as `["goIsekai/1.0", "default"]`
- **AND** it issues its first request
- **THEN** the request carries `User-Agent: goIsekai/1.0`

#### Scenario: Plugin without a list
- **WHEN** a plugin declares no `user_agents`
- **THEN** its requests carry the host's configured User-Agent

#### Scenario: Image fetches follow the plugin's list
- **WHEN** the host fetches a cover or page image for a plugin that declares `user_agents`
- **THEN** the fetch uses that plugin's current User-Agent, not the host default

### Requirement: User-Agent fallback on block
When a request is answered with a block response (a recognised challenge, a WAF block, or a non-image HTML response to an image request), the host SHALL retry it with the plugin's next untried `user_agents` entry before escalating to a browser engine. The first entry that yields a non-block response SHALL be remembered for that plugin for the life of the process, and SHALL be used for its subsequent requests. When every entry is blocked the host SHALL surface the challenge error.

#### Scenario: Second entry clears the block
- **WHEN** the first entry is blocked and the second returns a normal response
- **THEN** the host returns the second response to the caller
- **AND** later requests for that plugin start with the second entry

#### Scenario: Every entry blocked
- **WHEN** every declared entry returns a block response
- **THEN** the host surfaces the challenge error so the existing verify flow can proceed

#### Scenario: Resetting a plugin's profile
- **WHEN** the user resets a plugin's pinned TLS profile from the plugins page
- **THEN** the remembered User-Agent for that plugin is cleared as well

### Requirement: User-Agent precedence
The host SHALL resolve the `User-Agent` of a request in this order: the User-Agent seeded by a completed verification for that plugin, then a `User-Agent` header the plugin sets on that specific request, then the plugin's remembered or first declared entry, then the host default. The `Sec-CH-UA` hint SHALL follow the User-Agent actually sent and SHALL be omitted for a non-Chromium value.

#### Scenario: Verification clearance keeps its identity
- **WHEN** a plugin has cookies seeded from a verification together with a User-Agent
- **THEN** requests use that User-Agent even if the plugin declares `user_agents`

#### Scenario: Per-request header wins over the list
- **WHEN** a plugin sets a `User-Agent` header on one request
- **THEN** that request uses it and the declared list is not consulted

#### Scenario: Client hint follows the value
- **WHEN** the User-Agent sent is not Chromium-family
- **THEN** no `Sec-CH-UA` header is sent

### Requirement: Non-image response to an image request is a block
When a 2xx response to an image request has an HTML content type or a body that is not a decodable image, and the body is an HTML document, the host SHALL treat it as a block response rather than as corrupt image data. It SHALL NOT repeat the identical request, and it SHALL NOT cache the body.

#### Scenario: Edge interstitial served as 200
- **WHEN** an image URL answers 200 with an HTML verification page
- **THEN** the host moves to the next User-Agent entry or TLS profile instead of retrying the same request
- **AND** nothing is written to the image cache

#### Scenario: Genuinely corrupt image bytes
- **WHEN** a 2xx response is not HTML and does not decode as an image
- **THEN** the host keeps its existing corrupt-image retry behaviour

#### Scenario: Blocked image surfaces as a challenge
- **WHEN** every User-Agent entry and TLS profile returns the HTML interstitial
- **THEN** the image request fails with a challenge error, not an "invalid image data" error
