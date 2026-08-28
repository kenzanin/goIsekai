// Package hostnet provides the sandboxed host-side HTTP proxy. Plugins cannot
// open sockets directly; all network access flows through the host-imported
// host_http_request function implemented here, with standard header injection
// and per-plugin cookie persistence.
package hostnet
