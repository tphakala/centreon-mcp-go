package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// defaultHTTPPort is the listen port used when MCP_HTTP_PORT is unset.
const defaultHTTPPort = 8080

// URL schemes accepted for a Centreon host. These name a URL scheme and are
// deliberately separate from the transport* constants (which name the MCP
// transport mode), even though "http" is the same string in both.
const (
	schemeHTTPS = "https"
	schemeHTTP  = "http"
)

// Config holds the server configuration.
type Config struct {
	Host            string
	Username        string
	Password        string
	Token           string
	AllowSelfSigned bool
	AllowHTTP       bool
	Transport       string
	HTTPPort        int
	HTTPHost        string
	AuthMode        string
	LogLevel        string
	// AllowedHosts restricts the X-Centreon-Host header in gateway mode.
	// Empty means no restriction (any host accepted).
	AllowedHosts []string
}

// LoadConfig reads configuration from environment variables.
func LoadConfig() (Config, error) {
	cfg := Config{
		Host:     os.Getenv("CENTREON_HOST"),
		Username: os.Getenv("CENTREON_USERNAME"),
		Password: os.Getenv("CENTREON_PASSWORD"),
		Token:    os.Getenv("CENTREON_TOKEN"),
		HTTPHost: os.Getenv("MCP_HTTP_HOST"),
	}

	if cfg.Host == "" {
		return Config{}, fmt.Errorf("CENTREON_HOST environment variable is required")
	}

	// Either token or username+password must be set
	if cfg.Token == "" {
		if cfg.Username == "" {
			return Config{}, fmt.Errorf("CENTREON_USERNAME environment variable is required (or set CENTREON_TOKEN)")
		}
		if cfg.Password == "" {
			return Config{}, fmt.Errorf("CENTREON_PASSWORD environment variable is required (or set CENTREON_TOKEN)")
		}
	}

	cfg.Transport = envOr("MCP_TRANSPORT", transportStdio)
	cfg.LogLevel = envOr("LOG_LEVEL", "info")
	cfg.AuthMode = envOr("AUTH_MODE", authModeEnv)

	switch cfg.Transport {
	case transportStdio, transportHTTP:
	default:
		return Config{}, fmt.Errorf("invalid MCP_TRANSPORT value %q: expected stdio/http", cfg.Transport)
	}

	switch cfg.AuthMode {
	case authModeEnv, authModeGateway:
	default:
		return Config{}, fmt.Errorf("invalid AUTH_MODE value %q: expected env/gateway", cfg.AuthMode)
	}

	if cfg.HTTPHost == "" {
		cfg.HTTPHost = "0.0.0.0"
	}

	port, err := loadHTTPPort()
	if err != nil {
		return Config{}, err
	}
	cfg.HTTPPort = port

	cfg.AllowSelfSigned, err = parseBoolEnv("CENTREON_ALLOW_SELF_SIGNED")
	if err != nil {
		return Config{}, err
	}
	cfg.AllowHTTP, err = parseBoolEnv("CENTREON_ALLOW_HTTP")
	if err != nil {
		return Config{}, err
	}

	// Reject a cleartext http:// CENTREON_HOST (CWE-319) unless the operator opts
	// in. In HTTP gateway mode CENTREON_HOST is an unused placeholder and the real
	// hosts arrive per request via X-Centreon-Host, so its scheme is not validated
	// here (gatewayServer validates each header value instead).
	if !gatewayMode(&cfg) {
		if err := validateHostScheme(cfg.Host, cfg.AllowHTTP); err != nil {
			return Config{}, fmt.Errorf("CENTREON_HOST: %w", err)
		}
	}

	allowedHosts, err := loadAllowedHosts(cfg.AllowHTTP)
	if err != nil {
		return Config{}, err
	}
	cfg.AllowedHosts = allowedHosts

	return cfg, nil
}

// loadHTTPPort resolves the MCP_HTTP_PORT value, defaulting to 8080 when unset.
func loadHTTPPort() (int, error) {
	portStr := os.Getenv("MCP_HTTP_PORT")
	if portStr == "" {
		return defaultHTTPPort, nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid MCP_HTTP_PORT value %q: %w", portStr, err)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("MCP_HTTP_PORT must be between 1 and 65535, got %d", port)
	}
	return port, nil
}

// loadAllowedHosts reads and parses the CENTREON_ALLOWED_HOSTS allowlist.
// An unset or empty variable yields a nil slice (no restriction), matching
// os.Getenv, which cannot distinguish the two. A variable set to a non-empty
// value that has no valid entries after trimming (e.g. " , , ") is a
// misconfiguration and returns an error.
func loadAllowedHosts(allowHTTP bool) ([]string, error) {
	raw := os.Getenv("CENTREON_ALLOWED_HOSTS")
	if raw == "" {
		return nil, nil
	}
	hosts := parseAllowedHosts(raw)
	if len(hosts) == 0 {
		return nil, fmt.Errorf("CENTREON_ALLOWED_HOSTS is set but contains no valid host entries")
	}
	for _, h := range hosts {
		if err := validateHostScheme(h, allowHTTP); err != nil {
			return nil, fmt.Errorf("CENTREON_ALLOWED_HOSTS: %w", err)
		}
	}
	return hosts, nil
}

// parseAllowedHosts splits a comma-separated host list, trimming whitespace
// and dropping empty entries.
func parseAllowedHosts(raw string) []string {
	parts := strings.Split(raw, ",")
	hosts := make([]string, 0, len(parts))
	for _, p := range parts {
		if h := strings.TrimSpace(p); h != "" {
			hosts = append(hosts, h)
		}
	}
	return hosts
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// parseBoolEnv reads a boolean environment variable. An unset or empty value
// yields false with no error; any other value is parsed with strconv.ParseBool.
func parseBoolEnv(name string) (bool, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return false, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("invalid %s value %q: expected true/false", name, raw)
	}
	return v, nil
}

// gatewayMode reports whether the server runs in HTTP gateway mode: http
// transport with gateway auth. This is the only mode where the effective
// Centreon hosts arrive per request via the X-Centreon-Host header (so
// CENTREON_HOST is an unused placeholder) and the only mode that enforces the
// CENTREON_ALLOWED_HOSTS allowlist. Every other mode (stdio, or HTTP with env
// auth) uses CENTREON_HOST directly and never enforces the allowlist.
func gatewayMode(cfg *Config) bool {
	return cfg.Transport == transportHTTP && cfg.AuthMode == authModeGateway
}

// validateHostScheme rejects a Centreon host URL whose scheme would send
// credentials in cleartext. Credentials (username/password, or the X-AUTH-TOKEN
// header once authenticated) ride every request to the host, so an http:// target
// exposes them on the wire (CWE-319). https is always accepted; http only when the
// operator opts in via CENTREON_ALLOW_HTTP. A missing or non-http(s) scheme, or an
// unparseable URL, is rejected outright (fail closed). url.Parse normalises the
// scheme to lowercase, so the cases below are matched in lowercase.
//
// It also rejects an http(s) URL that parses but carries no hostname, which no
// caller can dial (issue #58). A raw '@' after the authority is NOT rejected:
// that shape is indistinguishable from a mis-encoded credential, so redaction in
// the log and the fail-closed placeholder in displayHost are the controls there,
// not validation (issue #55).
func validateHostScheme(host string, allowHTTP bool) error {
	u, err := url.Parse(host)
	if err != nil {
		// url.Error.Error() embeds the URL it failed on, so wrapping err directly
		// would reprint the raw credential that safeHost just masked in the same
		// message (CWE-532). Wrap only the reason.
		reason := err
		if urlErr, ok := errors.AsType[*url.Error](err); ok {
			reason = urlErr.Err
		}
		return fmt.Errorf("invalid host url %q: %w", safeHost(host), reason)
	}
	switch u.Scheme {
	case schemeHTTPS, schemeHTTP:
		// A parse can succeed while leaving no hostname to dial: "https:" and
		// "https:centreon.example.com" put everything in Opaque, "https://" has an
		// empty authority, and "https://admin:pass@" or "https://:8443" fill only
		// the userinfo or the port. Reject those here instead of letting
		// centreon.NewClient reject them, because its error formats the raw base
		// URL and that error is logged, which would put an embedded password in the
		// log in clear (issue #58, CWE-532).
		if u.Hostname() == "" {
			return fmt.Errorf("host %q must include a hostname (https://host.example[:port])", safeHost(host))
		}
		if u.Scheme == schemeHTTP && !allowHTTP {
			return fmt.Errorf("host %q uses http, which sends credentials in cleartext (CWE-319): use https or set CENTREON_ALLOW_HTTP=true", safeHost(host))
		}
		return nil
	case "":
		return fmt.Errorf("host %q must include a scheme (https://...)", safeHost(host))
	default:
		return fmt.Errorf("host %q has unsupported scheme %q: use https (or http with CENTREON_ALLOW_HTTP=true)", safeHost(host), u.Scheme)
	}
}

// safeHost masks the password in a host URL so it can be put into a log field or
// error message without leaking an embedded credential (CWE-532). It redacts the
// standard scheme://user:pass@host form via url.Redacted and the scheme-less
// user:pass@host form (which url.Parse treats as scheme:opaque, exposing no userinfo
// to mask) by reparsing it as an authority. url.Parse can also mis-read intended
// userinfo without failing: an all-numeric password prefix parses as a port, a short
// username parses as the host, and an email-style username makes the authority split
// land on the wrong '@', each leaving the credential in the path, query or fragment
// (issue #55). A host is returned unchanged only when no reading of it places a
// password span in the tail. Masking is textual once url.Parse has mis-read the
// input, so the masked host is the one the credential reading implies, which is not
// always the authority the client actually dialled.
//
// Limitation: a password that BEGINS with "://" behind a username that is itself a
// valid URL scheme (as in "admin://secret@host") is not masked, because that is
// byte-for-byte a scheme://authority URL whose userinfo is the username "secret"
// with no password at all, and safeHost never masks a username. A "//" anywhere
// else in the password, including the realistic base64 case, is handled. RFC 3986
// requires percent-encoding these characters in userinfo.
func safeHost(host string) string {
	if u, err := url.Parse(host); err == nil {
		if u.User != nil {
			if redactedCoversCredential(u, host) {
				return u.Redacted()
			}
		} else if u.Opaque == "" && u.Host != "" && !strings.HasSuffix(u.Host, ":") &&
			// A parsed URL with no userinfo can still be a mis-read credential:
			// url.Parse turns user:digits into host:port and a short username into
			// the host, leaving the '@' in the path, query or fragment (issue #55).
			// Returning unchanged is safe only when the input carries no raw '@' at
			// all, or when the parsed host is a bracketed IPv6 literal whose tail
			// holds no password span. '[' is an RFC 3986 gen-delim and invalid in
			// userinfo, so a bracketed authority cannot itself be a credential.
			(!strings.ContainsRune(host, '@') ||
				(strings.HasPrefix(u.Host, "[") && !tailCarriesPassword(host))) {
			return host
		}
	}
	// The scheme-less user:pass@host form parses as scheme:opaque, so reparse it as
	// an authority. Only the single-'@' form is unambiguous enough to trust: with a
	// second '@' the reparse invents an authority out of "scheme:user", masks the
	// wrong span, and leaves the real password in place.
	if strings.Count(host, "@") == 1 {
		if u, err := url.Parse("//" + host); err == nil && u.User != nil {
			return strings.TrimPrefix(u.Redacted(), "//")
		}
	}
	return maskAuthorityPassword(host)
}

// redactedCoversCredential reports whether url.Redacted masks the whole credential
// for a URL url.Parse decoded userinfo from. It does not when a ':' before a later
// '@' leaves a password span in the tail, nor when the userinfo carries no password
// and the host holds more than one '@', which means the authority split landed on
// an '@' that was part of the username (issue #55). Redacted only ever masks the
// password it parsed, and never looks at the path, query or fragment.
func redactedCoversCredential(u *url.URL, host string) bool {
	if tailCarriesPassword(host) {
		return false
	}
	if _, hasPassword := u.User.Password(); hasPassword {
		return true
	}
	return strings.Count(host, "@") == 1
}

// authorityTail returns everything from the first '/', '?' or '#' that follows the
// authority marker. That is not always url.Parse's own path/query/fragment split,
// and deliberately so: the inputs that matter here are exactly the ones url.Parse
// splits in the wrong place, so this works on the raw string instead.
func authorityTail(host string) string {
	rest := host
	if k := authorityMarker(rest); k >= 0 {
		rest = rest[k:]
	}
	if j := strings.IndexAny(rest, "/?#"); j >= 0 {
		return rest[j:]
	}
	return ""
}

// authorityMarker returns the index just past the "//" that introduces the
// authority, or -1 when host has none. Only a leading "//" or one preceded by a
// syntactically valid scheme counts. Accepting any "//" instead would let a
// password containing one pose as the authority marker, which swallowed the real
// password span and returned the host unmasked.
func authorityMarker(host string) int {
	const separator = "//"
	i := strings.Index(host, separator)
	switch {
	case i < 0:
		return -1
	case i == 0:
		return len(separator)
	case host[i-1] == ':' && isURLScheme(host[:i-1]):
		return i + len(separator)
	default:
		return -1
	}
}

// isURLScheme reports whether s is a valid RFC 3986 scheme: ALPHA followed by any
// of ALPHA, DIGIT, '+', '-' and '.'. The empty string is accepted so the malformed
// but already-pinned "://host" form keeps its authority marker.
func isURLScheme(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		case i > 0 && (c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.'):
		default:
			return false
		}
	}
	return true
}

// tailCarriesPassword reports whether the tail holds a ':' before a later '@', the
// shape of a password span url.Parse did not decode as userinfo.
func tailCarriesPassword(host string) bool {
	tail := authorityTail(host)
	at := strings.LastIndexByte(tail, '@')
	return at >= 0 && strings.IndexByte(tail[:at], ':') >= 0
}

// displayHost reduces a host URL to scheme://host[:port] for display in a tool
// response, stripping ALL userinfo (username and password) plus any path, query
// and fragment, so no credential embedded in the URL reaches a client. This is
// stricter than safeHost, which keeps the username for log greps: issue #48
// requires the response to expose no username, password or token. Every
// display-path caller first runs the host through validateHostScheme, which
// guarantees a parseable http/https URL, so the fail-closed placeholder is a
// defensive fallback that never fires for a validated host.
// redactedHostPlaceholder is the fail-closed display value for a host that has no
// usable hostname, so a malformed or hostless input is never echoed verbatim.
const redactedHostPlaceholder = "(redacted host)"

func displayHost(host string) string {
	if u, err := url.Parse(host); err == nil && u.Hostname() != "" {
		return u.Scheme + "://" + u.Host
	}
	return redactedHostPlaceholder
}

// maskAuthorityPassword is the textual fail-closed fallback for host strings whose
// userinfo url.Parse either rejects (a password with an unescaped '/', '?' or '#',
// a leading "://", an invalid percent-escape) or silently mis-reads as host, port,
// path, query or fragment (issue #55). It masks the span from the first ':' after
// the authority marker to the last '@' (the userinfo/host delimiter). A string with
// no '@', or with no ':' between the authority marker and the last '@', has no
// password span and is returned unchanged.
func maskAuthorityPassword(host string) string {
	rest, prefix := host, ""
	if k := authorityMarker(rest); k >= 0 {
		prefix, rest = rest[:k], rest[k:]
	}
	at := strings.LastIndexByte(rest, '@')
	if at < 0 {
		return host // no userinfo
	}
	userinfo := rest[:at]
	colon := userinfoColon(userinfo)
	if colon < 0 {
		return host // no ':' before the delimiter, so there is no password span
	}
	return prefix + userinfo[:colon] + ":xxxxx" + rest[at:]
}

// userinfoColon returns the index of the ':' that starts the password span, or -1.
// A leading bracketed IPv6 literal is skipped so its own colons are not mistaken
// for the delimiter, which would otherwise mask from inside the address and leave
// a truncated "[" where the host should be. The literal must parse as an IP, so a
// bracket-prefixed username such as "[user:secret" still masks at its real colon.
func userinfoColon(userinfo string) int {
	from := 0
	if strings.HasPrefix(userinfo, "[") {
		if end := strings.IndexByte(userinfo, ']'); end > 0 {
			literal := userinfo[1:end]
			// A zone ID ("%25eth0" once percent-encoded) is not part of the address.
			if pct := strings.IndexByte(literal, '%'); pct >= 0 {
				literal = literal[:pct]
			}
			if net.ParseIP(literal) != nil {
				from = end + 1
			}
		}
	}
	colon := strings.IndexByte(userinfo[from:], ':')
	if colon < 0 {
		return -1
	}
	return from + colon
}
