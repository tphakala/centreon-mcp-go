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
	// ReadOnly gates the mutating tools. When true (MCP_READ_ONLY=true), tools
	// that write to Centreon are still listed but refuse with a tool-level error.
	ReadOnly  bool
	Transport string
	HTTPPort  int
	HTTPHost  string
	AuthMode  string
	LogLevel  string
	// AllowedHosts restricts the X-Centreon-Host header in gateway mode.
	// Empty means no restriction (any host accepted).
	AllowedHosts []string
}

// loadCredentials resolves the password and token, preferring their _FILE
// variants when set, and validates that the host and a usable credential
// (a token, or a username plus password) are present.
func loadCredentials(cfg *Config) error {
	password, err := resolveSecretEnv("CENTREON_PASSWORD")
	if err != nil {
		return err
	}
	cfg.Password = password
	token, err := resolveSecretEnv("CENTREON_TOKEN")
	if err != nil {
		return err
	}
	cfg.Token = token

	if cfg.Host == "" {
		return fmt.Errorf("CENTREON_HOST environment variable is required")
	}

	// Either token or username+password must be set
	if cfg.Token == "" {
		if cfg.Username == "" {
			return fmt.Errorf("CENTREON_USERNAME environment variable is required (or set CENTREON_TOKEN)")
		}
		if cfg.Password == "" {
			return fmt.Errorf("CENTREON_PASSWORD environment variable is required (or set CENTREON_TOKEN)")
		}
	}
	return nil
}

// LoadConfig reads configuration from environment variables.
func LoadConfig() (Config, error) {
	cfg := Config{
		Host:     os.Getenv("CENTREON_HOST"),
		Username: os.Getenv("CENTREON_USERNAME"),
		HTTPHost: os.Getenv("MCP_HTTP_HOST"),
	}

	if err := loadCredentials(&cfg); err != nil {
		return Config{}, err
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
	cfg.ReadOnly, err = parseBoolEnv("MCP_READ_ONLY")
	if err != nil {
		return Config{}, err
	}

	// Reject a CENTREON_HOST that names no hostname, or that is cleartext http://
	// without the operator's opt-in (CWE-319). In HTTP gateway mode CENTREON_HOST is
	// an unused placeholder and the real hosts arrive per request via
	// X-Centreon-Host, so it is not validated here at all (gatewayServer validates
	// each header value instead).
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

// resolveSecretEnv returns the value of the secret environment variable name,
// preferring the file named by the <name>_FILE variant when that variant is
// set. The file form (Docker and Kubernetes secrets convention) takes
// precedence over the inline variable so a mounted secret cannot be shadowed
// by a stale inline value. A set but missing, unreadable, or empty file is a
// hard configuration error: failing fast beats authenticating with an empty
// secret. The error names the variable and the file path but never the file
// contents (CWE-532).
func resolveSecretEnv(name string) (string, error) {
	fileVar := name + "_FILE"
	path := os.Getenv(fileVar)
	if path == "" {
		return os.Getenv(name), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%s: cannot read secret file: %w", fileVar, err)
	}
	secret := trimTrailingNewline(string(data))
	if secret == "" {
		return "", fmt.Errorf("%s: secret file %q is empty", fileVar, path)
	}
	return secret, nil
}

// trimTrailingNewline removes exactly one trailing line break, "\r\n" or "\n",
// from s. Nothing else is trimmed: a secret may legitimately end in a space,
// a tab, or even a bare carriage return, so only the newline that an editor
// or an echo appends is stripped, and only one of them.
func trimTrailingNewline(s string) string {
	if after, ok := strings.CutSuffix(s, "\r\n"); ok {
		return after
	}
	if after, ok := strings.CutSuffix(s, "\n"); ok {
		return after
	}
	return s
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
// exposes them on the wire (CWE-319). https is accepted when it names a host; http
// only when the operator opts in via CENTREON_ALLOW_HTTP. A missing or non-http(s)
// scheme, or an unparseable URL, is rejected outright (fail closed). url.Parse
// normalises the scheme to lowercase, so the cases below are matched in lowercase.
//
// It also rejects an http(s) URL that parses but names no hostname (issue #58). A
// raw '@' after the authority is NOT rejected:
// that shape is indistinguishable from a mis-encoded credential, so redaction in
// the log and the fail-closed placeholder in displayHost are the controls there,
// not validation (issue #55).
// urlParseReason maps a url.Parse failure to a fixed description of what was wrong.
// It never copies a string out of the error, because Go's parse reasons quote the
// offending span of the input, which for a host URL is the credential. The returned
// value is always one of the constants below, so no input can reach a log through
// it. This is the same fail-closed philosophy as internal/redact.Reason, which does
// the job for client-call errors, applied to the one parse error that predates it.
func urlParseReason(err error) string {
	reason := err
	if urlErr, ok := errors.AsType[*url.Error](err); ok {
		reason = urlErr.Err
	}
	switch text := reason.Error(); {
	case strings.HasPrefix(text, "invalid port "):
		return "invalid port after host"
	case strings.HasPrefix(text, "invalid URL escape "):
		return "invalid percent-escape"
	case strings.HasPrefix(text, "invalid character "):
		return "invalid character in host name"
	default:
		return "malformed URL"
	}
}

func validateHostScheme(host string, allowHTTP bool) error {
	u, err := url.Parse(host)
	if err != nil {
		// url.Error.Error() embeds the URL it failed on, so wrapping err directly
		// would reprint the raw credential that safeHost just masked in the same
		// message (CWE-532). Unwrapping one layer is not enough: Go's own reason for
		// a mis-parsed authority is `invalid port ":<password>" after host`, which
		// quotes the credential too, so the masked host and the password shipped in
		// the same log line. Classify instead of quoting.
		return fmt.Errorf("invalid host url %q: %s", safeHost(host), urlParseReason(err))
	}
	switch u.Scheme {
	case schemeHTTPS, schemeHTTP:
		// A parse can succeed while naming no hostname, in two groups that are
		// rejected for different reasons (issue #58).
		//
		// Host is empty: "https:" and "https:centreon.example.com" leave everything
		// in Opaque, "https://" has an empty authority, and "https://admin:pass@"
		// fills only the userinfo. centreon.NewClient rejects these itself, with an
		// error that formats the RAW base URL, and that error is logged, so an
		// embedded password would reach the log in clear (CWE-532).
		//
		// Host is non-empty but holds only a port, as in "https://:8443". The client
		// accepts this one and Go's dialer would treat it as localhost, so it is
		// rejected as a configuration error rather than a leak: a base URL that names
		// no host is a typo, and https cannot verify a certificate without a name.
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
//
// Where a credential is present but its boundary is only visible after decoding,
// safeHost fails closed and drops the host rather than echo the input: a delimiter
// whose hex digits are encoded (issue #68) and an encoded ':' separator, which Go
// reads as a bare username so url.Redacted masks nothing (issue #75).
//
// That rule is not an absolute, and the exceptions are the interesting part. A
// credential needs BOTH a delimiter and a separator, so an input carrying only one
// of them is left intact: "https://host/path/%25%34%30" decodes to a '@' and hides
// nothing. The username is kept when a raw colon marks where the password starts,
// giving "https://admin:xxxxx" instead of "https://xxxxx"; that is the usual shape
// for issue #68, while issue #75 mostly loses the username too, since an encoded
// separator is exactly the case where no raw colon marks the boundary. And the
// bounded decode reports exhaustion as uncertainty rather
// than as a finding, so an ordinary path nested deeper than maxDecodeRounds is left
// alone unless something else in the input actually looks like a credential.
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
			// Returning unchanged is safe only when the input carries no at sign that
			// could be a userinfo delimiter, raw OR percent-encoded as %40, which
			// tailCarriesAtSign decodes (issue #62); or when the parsed host is a
			// bracketed IPv6 literal whose tail holds no password span. '[' is an RFC
			// 3986 gen-delim and invalid in userinfo, so a bracketed authority cannot
			// itself be a credential. afterAuthorityDelimiter (not authorityTail) is
			// used so an encoded '@' sitting in the authority of a host:port form, with
			// no '/', '?' or '#' after it, is still seen.
			((!strings.ContainsRune(host, '@') && !tailCarriesAtSign(afterAuthorityDelimiter(host))) ||
				(strings.HasPrefix(u.Host, "[") && !tailCarriesPassword(host))) {
			return host
		}
	}
	// The scheme-less user:pass@host form parses as scheme:opaque, so it needs a
	// reparse; anything the reparse cannot read unambiguously falls through to the
	// textual masker below.
	if masked, ok := reparsedAuthority(host); ok {
		return masked
	}
	return maskAuthorityPassword(host)
}

// reparsedAuthority masks the scheme-less user:pass@host form, which url.Parse
// reads as scheme:opaque and so exposes no userinfo to mask. It reports false when
// the form is too ambiguous to trust, leaving the caller on the textual masker.
//
// Only the single-'@' form with no further at sign after the delimiter is
// unambiguous enough: a second '@' (raw, or percent-encoded as %40 in the tail)
// means the reparse would invent an authority out of "scheme:user" or mask the
// wrong span and leave the real password in place (issues #55, #62). The
// encoded-separator check is the one redactedCoversCredential makes, for the same
// reason: this hands the string to url.Redacted, which masks nothing when Go read
// the userinfo as a bare username because its ':' was percent-encoded (issue #75).
func reparsedAuthority(host string) (string, bool) {
	if strings.Count(host, "@") != 1 {
		return "", false
	}
	u, err := url.Parse("//" + host)
	if err != nil || u.User == nil ||
		tailCarriesAtSign(afterAuthorityDelimiter(host)) ||
		decodesToContain(u.User.Username(), ':') {
		return "", false
	}
	return strings.TrimPrefix(u.Redacted(), "//"), true
}

// redactedCoversCredential reports whether url.Redacted masks the whole credential
// for a URL url.Parse decoded userinfo from. It does not when an at sign, raw or
// percent-encoded as %40, survives after the '@' url.Parse split the authority at:
// the split may then have landed inside the password and left the rest in the host,
// path, query or fragment (issues #55, #62). It also does not when the userinfo
// carries no password and the host holds more than one '@', which means the split
// landed on an '@' that was part of the username. Redacted only ever masks the
// password it parsed, and never looks past the authority it chose.
func redactedCoversCredential(u *url.URL, host string) bool {
	if tailCarriesAtSign(afterAuthorityDelimiter(host)) {
		return false
	}
	// Go splits userinfo on a RAW ':' only, so an encoded separator ("%3A") is not a
	// boundary to it (issue #75). Username() returns the decoded span, so a ':' in it
	// is exactly that case, and it must be ruled out BEFORE either check below.
	//
	// Ahead of hasPassword, because an encoded separator EARLIER than the raw one Go
	// split on means the real password starts sooner: url.Redacted then masks only
	// the fragment after the raw ':' and echoes the rest. "admin%3Apw:x@host" masked
	// to "admin%3Apw:xxxxx@host" and published the password.
	//
	// Ahead of the '@' count, because that fallback is an exemption concluding "a
	// bare username with nothing to hide", and an exemption that checks less than
	// the whole span it exempts is a leak (the lesson of issue #62).
	if decodesToContain(u.User.Username(), ':') {
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

// afterAuthorityDelimiter returns the span of host that follows the '@' url.Parse
// takes as the userinfo/host delimiter: the last raw '@' inside the authority (the
// text between the "//" marker and the first '/', '?' or '#'). It differs from
// authorityTail, which starts at that first '/', '?' or '#' and so cannot see the
// host remnant a mis-read leaves before it ("cret" in "admin:se@cret/path@host").
// With no raw '@' in the authority it returns the whole post-marker span, so an
// encoded delimiter further along is still in view. This reproduces Parse's own
// split point textually, which is what lets safeHost tell a decoded password that
// Redacted fully covers from one it only partly covers (issue #62).
func afterAuthorityDelimiter(host string) string {
	rest := host
	if k := authorityMarker(rest); k >= 0 {
		rest = rest[k:]
	}
	auth := rest
	if j := strings.IndexAny(rest, "/?#"); j >= 0 {
		auth = rest[:j]
	}
	if at := strings.LastIndexByte(auth, '@'); at >= 0 {
		return rest[at+1:]
	}
	return rest
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
// shape of a password span url.Parse did not decode as userinfo. The '@' may be
// percent-encoded (%40), so the delimiter is located with lastAtDelimiter rather
// than a raw-byte search; this closes the same hole in a bracketed-IPv6 tail that
// the encoded-delimiter fix closes elsewhere (issue #62).
func tailCarriesPassword(host string) bool {
	tail := authorityTail(host)
	at := lastAtDelimiter(tail)
	if at < 0 {
		// lastAtDelimiter cannot see a delimiter whose hex digits are encoded, so the
		// bracketed-authority carve-out above used to return such an input verbatim
		// (issue #68). The position is unknown here, so ask only whether a separator
		// is positively present anywhere in the tail.
		sep, _ := decodesToTarget(tail, ':')
		return decodesToContain(tail, '@') && sep
	}
	// The separator is sought in every encoding, not just raw. A raw IndexByte here
	// let the same carve-out return an input verbatim whenever the tail's ':' was
	// written "%3A", the hazard the masker itself was fixed for (issue #75).
	return decodesToContain(tail[:at], ':')
}

// redactedHostPlaceholder is the fail-closed display value for a host whose
// authority cannot be trusted, so a malformed, hostless or mis-parsed input is
// never echoed verbatim.
const redactedHostPlaceholder = "(redacted host)"

// displayHost reduces a host URL to scheme://host[:port] for display in a tool
// response, stripping ALL userinfo (username and password) plus any path, query
// and fragment. It is stricter than safeHost, which keeps the username for log
// greps: issue #48 requires the response to expose no username, password or token.
//
// url.Parse ends the authority at the first '/', '?' or '#', so an unencoded one
// inside userinfo makes it read part of the credential as the host:
// "https://admin:1234/secret@centreon.example.com" parses with host "admin:1234",
// where "1234" is the first segment of the password the operator typed, and
// "https://admin:p@ssword/x@centreon.example.com" parses with host "ssword"
// (issue #57). The tell in both is a '@' that survives in the span afterAuthorityDelimiter
// returns, raw or percent-encoded to any depth, which tailCarriesAtSign decides. That span
// includes the authority url.Parse chose, not only the path/query/fragment tail: "%25" is
// a legal host character sequence, so a delimiter encoded twice ("%2540") can survive
// INSIDE u.Host with no '/', '?' or '#' after it, where a tail-only check never looks
// (issue #70). A raw '@' inside the authority itself needs no guard: url.Parse splits at
// the last one, so the earlier ones belong to the userinfo it decoded (they may land in
// the username rather than the password).
//
// Failing closed on that tell also catches a legitimate '@' in a path, query or
// fragment, which no rule can tell apart from a mis-encoded credential without
// resolving names. displayHost discards all three regardless, so only the host name
// is lost. A bracketed authority is exempt: url.Parse requires the address part of
// its body to parse as an IP literal, so it is the host and not a mis-read
// credential: url.Parse rejects a '[' anywhere but the start, requires the address
// to parse, and validates an RFC 6874 "%25" zone too, refusing an empty one or an
// escaped byte inside it. Note the echoed zone has its "%25" decoded to a bare '%',
// so an output naming a zone is not itself a reparseable URL.
//
// A URL with no scheme also fails closed, so the result is never a bare "://host".
// validateHostScheme rejects a missing scheme before any display-path caller is
// reached, so this only matters to a direct caller.
func displayHost(host string) string {
	u, err := url.Parse(host)
	if err != nil || u.Scheme == "" || u.Hostname() == "" {
		return redactedHostPlaceholder
	}
	if strings.HasPrefix(u.Host, "[") {
		return u.Scheme + "://" + u.Host
	}
	if tailCarriesAtSign(afterAuthorityDelimiter(host)) {
		return redactedHostPlaceholder
	}
	return u.Scheme + "://" + u.Host
}

// tailCarriesAtSign reports whether what follows an authority still holds a '@',
// the tell that url.Parse may have ended the authority inside a credential rather
// than at its delimiter.
//
// The '@' can arrive percent-encoded, and encoded more than once ("%2540"), so a
// substring test for "%40" is not enough: the tail is unescaped repeatedly until no
// escape is left. Every exit other than "no '@' and nothing left to unescape" fails
// closed, because a tail that will not resolve cannot be ruled out as an encoded
// delimiter. Three things reach that verdict: a malformed escape such as "%4%30", a
// tail still escaped when the bound runs out, and a legitimate encoded percent sign,
// since unescaping "%25" yields a bare '%' that the next pass cannot resolve. The
// last of those is the only one an operator is likely to meet, in a path or query
// such as "/100%25". The fixed-point exit is defensive only: a successful unescape of
// a string containing '%' always shortens it.
//
// Unescaping rather than scanning for '%' is what keeps an ordinary encoded path such
// as "/mon%20test" trusted, along with "%2F", "%3D" and a UTF-8 escape like "%C3%A9".
// Note "%4c" is a valid escape for 'L', so a tail like "/pw%4chost" holds no
// delimiter in any encoding and is trusted too.
func tailCarriesAtSign(tail string) bool {
	// Shares maxDecodeRounds with decodesToContain so the routing guards and the
	// textual masker agree on how deep a delimiter may hide. The bound used to be a
	// bare literal here while the constant's doc claimed they matched, which nothing
	// enforced: changing one silently desynchronised the two.
	for range maxDecodeRounds {
		if strings.ContainsRune(tail, '@') {
			return true
		}
		if !strings.ContainsRune(tail, '%') {
			return false
		}
		next, err := url.PathUnescape(tail)
		if err != nil || next == tail {
			return true
		}
		tail = next
	}
	return true
}

// maskAuthorityPassword is the textual fail-closed fallback for host strings whose
// userinfo url.Parse either rejects (a password with an unescaped '/', '?' or '#',
// a leading "://", an invalid percent-escape) or silently mis-reads as host, port,
// path, query or fragment (issue #55). It masks the span from the first ':' after
// the authority marker to the delimiter '@', found by lastAtDelimiter so a
// percent-encoded delimiter (%40) is located too (issue #62). A string with no
// delimiter, or with no ':' before it in any encoding, has no password span and is
// returned unchanged. The encoded delimiter is kept verbatim in the output, so the
// host after it still greps.
//
// Two shapes cannot be masked in place and fail closed through
// maskAmbiguousAuthority instead: a delimiter whose hex digits are themselves
// encoded (issue #68), and a userinfo whose ':' separator is encoded (issue #75).
// Both are locatable only after decoding, and mapping a decoded offset back through
// every round is not worth the complexity, so the position is given up rather than
// reconstructed. Note the two differ in reachability: validateHostScheme rejects
// the #68 shapes outright, so in env mode only a caller-supplied X-Centreon-Host
// header produces them, while it ACCEPTS "https://admin%3Apw@host", which therefore
// runs normally and reaches a log on every line.
func maskAuthorityPassword(host string) string {
	rest, prefix := host, ""
	if k := authorityMarker(rest); k >= 0 {
		prefix, rest = rest[:k], rest[k:]
	}
	at := lastAtDelimiter(rest)
	// lastAtDelimiter sees a raw '@' and the flat %(25)*40 shape. It cannot see a
	// delimiter whose own hex digits are encoded ("%25%34%30" decodes to "%40" and
	// then to '@'), and such a delimiter can sit AFTER the one it did find, leaving
	// the span between them standing as password material (issue #68). Its position
	// is only recoverable by mapping an offset back through every decode round, so
	// this fails closed instead and masks from the userinfo separator.
	//
	// A delimiter alone is not a credential. Requiring a separator too is what keeps
	// an ordinary URL intact: "https://host/path/%25%34%30" decodes to a '@' and
	// holds no secret, and decodesToContain deliberately fails closed once its round
	// bound runs out, which "https://host/100%25252525" reaches with no '@' at all.
	// Masking either of those would drop a hostname an operator needs, to hide
	// nothing.
	deeper := rest
	if at >= 0 {
		deeper = rest[at+1:]
	}
	if decodesToContain(deeper, '@') {
		// The separator must be positively found, not merely possible: ambiguity is
		// not evidence, and the two searches exhaust the round bound together on a
		// deeply nested path that holds neither character.
		if sep, _ := decodesToTarget(rest, ':'); !sep {
			return host // a delimiter but no separator, so no password span
		}
		return maskAmbiguousAuthority(prefix, rest)
	}
	if at < 0 {
		return host // no userinfo
	}
	// The separator can itself be percent-encoded ("%3A"), in which case Go parsed a
	// bare username and url.Redacted masked nothing (issue #75). Search for it over
	// EXACTLY the span the raw search covers: past a leading bracketed IP literal,
	// whose own colons belong to the address, and ahead of the first raw colon,
	// because a colon after it is a port or tail colon and masking there would leave
	// the real credential standing in front of the mask.
	userinfo := rest[:at]
	from, ok := userinfoSearchStart(userinfo)
	if !ok {
		return host // the userinfo is a bare bracketed IP literal, so no password span
	}
	searched := userinfo[from:]
	colon := strings.IndexByte(searched, ':')
	encoded := searched
	if colon >= 0 {
		encoded = searched[:colon]
	}
	if decodesToContain(encoded, ':') {
		return maskAmbiguousAuthority(prefix, rest)
	}
	if colon < 0 {
		return host // no ':' before the delimiter, so there is no password span
	}
	return prefix + userinfo[:from+colon] + ":xxxxx" + rest[at:]
}

// maskAmbiguousAuthority masks from the password span to the end of the input, for
// the case where a credential is present but its delimiter cannot be located in the
// original bytes. A username is not a secret and keeps the line greppable, so it is
// preserved when a raw colon identifies one; when even the colon is visible only
// after decoding, there is no trustworthy boundary and the whole authority goes.
//
// "Identifies one" is the load-bearing phrase. rest spans the whole authority, so a
// raw colon found past the credential delimiter is a port or an IPv6 literal's own
// colon, and masking from there returns the userinfo, and the credential, verbatim.
// A first attempt at this function got that wrong and masked the port of any host
// that reached it carrying one, echoing the userinfo in front. It was caught in
// review and never shipped, but it is the reason the guard is spelled out here.
func maskAmbiguousAuthority(prefix, rest string) string {
	// rest spans the whole authority, so the first colon found in it is only a
	// username boundary when no credential delimiter precedes it. Past a delimiter it
	// is the PORT colon, and masking there would return the userinfo verbatim.
	//
	// The span kept as the username has to be innocent on two counts. No credential
	// delimiter may precede the colon, or the colon belongs to a port rather than to
	// a boundary. And the searched region may hold no separator of its own under an
	// encoding, or the span IS the credential: in "admin%3Apw:x@host" everything
	// before the raw ':' is the real user and password.
	//
	// The second check covers rest[from:colon], not rest[:colon], because a bracketed
	// literal skipped by the search carries its own raw colons and they are part of
	// an address, not a boundary.
	from, ok := userinfoSearchStart(rest)
	if !ok {
		return prefix + "xxxxx"
	}
	colon := strings.IndexByte(rest[from:], ':')
	if colon < 0 {
		return prefix + "xxxxx"
	}
	colon += from
	if decodesToContain(rest[:colon], '@') || decodesToContain(rest[from:colon], ':') {
		return prefix + "xxxxx"
	}
	return prefix + rest[:colon] + ":xxxxx"
}

// maxDecodeRounds bounds how many rounds of percent-decoding decodesToContain
// follows before giving up. It matches the bound in tailCarriesAtSign so the
// routing guards and the textual masker agree on how deep a delimiter may hide
// before an input is treated as ambiguous.
const maxDecodeRounds = 4

// decodesToContain reports whether s decodes to a string containing target within
// maxDecodeRounds rounds of decodeLenient. It fails closed, returning true, when
// the rounds run out while decoding is still making progress, because target could
// be hiding under another layer. Decoding that stops making progress is complete,
// so a string whose remaining escapes are undecodable (a "%zz" typo in a path) is
// answered from its decoded form rather than being treated as suspicious.
//
// Allocation is bounded at maxDecodeRounds strings of at most len(s) bytes each,
// independent of how deeply the input nests. Peeling one layer per candidate
// position instead was measured at 1.15GB per call on a 64KB nested input, so the
// bound is deliberately on rounds rather than on layers (issue #62).
func decodesToContain(s string, target byte) bool {
	found, ambiguous := decodesToTarget(s, target)
	return found || ambiguous
}

// decodesToTarget is decodesToContain with its two reasons kept apart: found means
// target actually appeared within the bound, ambiguous means the rounds ran out
// while decoding was still making progress, so target could be hiding deeper.
//
// The distinction matters wherever ambiguity alone must not drive a decision. A
// credential needs both a delimiter and a separator, and a deeply nested but
// perfectly ordinary path ("/100%25252525") exhausts the bound on BOTH searches at
// once, so treating ambiguity as evidence for each of them in turn would destroy
// the hostname to hide nothing.
func decodesToTarget(s string, target byte) (found, ambiguous bool) {
	for range maxDecodeRounds {
		if strings.IndexByte(s, target) >= 0 {
			return true, false
		}
		next := decodeLenient(s)
		if next == s {
			return false, false // decoding is complete and target never appeared
		}
		s = next
	}
	return false, true
}

// decodeLenient applies one round of percent-decoding to s. An escape that is not
// '%' followed by two hex digits is left as a literal '%' and scanning continues
// past it. url.PathUnescape cannot serve here because it rejects the whole string
// on the first bad escape, which would let a "%zz" typo anywhere in a URL hide a
// credential delimiter elsewhere in it. s is returned unchanged, without
// allocating, when it holds nothing decodable.
// Literal runs between escapes are copied whole rather than a byte at a time, and
// decoding starts at the first escape rather than rescanning the prefix the search
// already walked. In gateway mode the input is a caller-supplied header, so this is
// the difference between copying at memmove speed and at one call per byte on an
// input an attacker chooses the length of.
func decodeLenient(s string) string {
	first := firstEscape(s)
	if first < 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	run := 0
	for i := first; i < len(s); {
		if isEscapeAt(s, i) {
			b.WriteString(s[run:i])
			b.WriteByte(unhexDigit(s[i+1])<<4 | unhexDigit(s[i+2]))
			i += 3
			run = i
			continue
		}
		i++
	}
	b.WriteString(s[run:])
	return b.String()
}

// firstEscape returns the index of the first '%' followed by two hex digits, which
// is what decodeLenient rewrites, or -1 when s holds none.
func firstEscape(s string) int {
	for i := range len(s) {
		if isEscapeAt(s, i) {
			return i
		}
	}
	return -1
}

// isEscapeAt reports whether a well-formed percent-escape starts at s[i].
func isEscapeAt(s string, i int) bool {
	return s[i] == '%' && i+2 < len(s) && isHexDigit(s[i+1]) && isHexDigit(s[i+2])
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// decimalDigits is how many digit characters precede 'a' and 'A' in the hex
// alphabet, so a letter's value is its distance from that letter plus this.
const decimalDigits = 10

func unhexDigit(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + decimalDigits
	default:
		return c - 'A' + decimalDigits
	}
}

// userinfoSearchStart returns the offset in userinfo at which a username/password
// separator may legitimately begin, skipping a leading bracketed IP literal whose
// own colons belong to the address rather than to a credential. ok is false when
// the whole span is such a literal, which carries no password span at all.
//
// It exists as a separate function because the encoded-separator search (issue #75)
// has to cover EXACTLY the span this offset opens, no more and no less. Searching
// the whole userinfo instead reads a bracketed literal's own colons as a separator,
// so a userinfo that IS such a literal ("https://[::1]@host") gets masked away for
// nothing; searching past the raw colon instead reports a port or tail colon and
// leaves the real credential standing in front of the mask. The skip is
// load-bearing in both callers: "https://[::1]/p:pw%25%34%30host" masks to
// "https://[::1]/p:xxxxx" with it and to "https://[:xxxxx" without.
func userinfoSearchStart(userinfo string) (from int, ok bool) {
	if !strings.HasPrefix(userinfo, "[") {
		return 0, true
	}
	if end := strings.IndexByte(userinfo, ']'); end > 0 {
		literal := userinfo[1:end]
		// A zone ID ("%25eth0" once percent-encoded) is not part of the address.
		if pct := strings.IndexByte(literal, '%'); pct >= 0 {
			literal = literal[:pct]
		}
		if net.ParseIP(literal) != nil {
			return end + 1, true
		}
		return 0, true
	}
	if net.ParseIP(userinfo[1:]) != nil {
		// No closing ']': the delimiter the caller found sits inside the bracket
		// because url.Parse rejected the authority (an invalid '%40' zone lands
		// here, issue #62) and the ']' fell past it. Only when the ENTIRE span
		// after '[' is a valid IP is it the host literal with no password to mask.
		// net.ParseIP rejects a zone or a ':' after the address, so a span like
		// "[192.168.0.1%x:secret" is NOT taken as bare host: it falls through and
		// its ':secret' is masked. (Stripping at '%' first would misread that as a
		// zoned address and leak the password.)
		return 0, false
	}
	return 0, true
}

// lastAtDelimiter returns the index in s of the position that acts as the
// userinfo/host delimiter: the last raw '@', or a later percent-escape that encodes
// '@' through any number of layers ("%40", "%2540", "%252540", ...). It returns -1
// when neither is present. A raw '@' after such an escape wins, matching url.Parse,
// which splits at the last raw '@'; an escape after the last raw '@' is the
// dangerous case a raw search misses (the hybrid "admin:p@ssword%40host", issue
// #62). Escapes that do not provably decode to '@' (a malformed "%zz", or "%4c" for
// 'L') are not delimiters, so an ordinary encoded path is left intact.
//
// This scan is deliberately incomplete. It recognises only the shape whose hex
// digits stay literal, because that one can be matched in place, allocation-free,
// at any depth. A delimiter whose digits are themselves encoded ("%25%34%30",
// issue #68) is invisible here; callers pair this with decodesToContain, which
// answers whether such a delimiter exists but not where, and so fails closed.
func lastAtDelimiter(s string) int {
	at := strings.LastIndexByte(s, '@')
	for i := len(s) - 1; i > at; i-- {
		if s[i] == '%' && escapeEncodesAt(s[i:]) {
			return i
		}
	}
	return at
}

// escapeEncodesAt reports whether s begins with a percent-escape that decodes to
// '@', including through nested encoding of any depth. '@' is 0x40, written "%40";
// each further encoding layer wraps the leading '%' (0x25) as "%25", so the only
// strings that decode to a leading '@' are "%40", "%2540", "%252540", and so on: a
// '%', then zero or more literal "25", then "40". Both bytes have a unique two-hex
// spelling ("40" and "25" contain no letters, so there is no case variant), so this
// one forward scan is EXACTLY the layer-by-layer decode, and unlike peeling the
// escape (which rebuilt the string each round) it allocates nothing and stays linear
// on a crafted input. That matters because safeHost runs on an attacker-influenced
// gateway X-Centreon-Host header (issue #62); an allocating peel was quadratic.
func escapeEncodesAt(s string) bool {
	if len(s) < 3 || s[0] != '%' {
		return false
	}
	i := 1
	for i+2 <= len(s) && s[i] == '2' && s[i+1] == '5' {
		i += 2 // peel one "%25" layer
	}
	return i+2 <= len(s) && s[i] == '4' && s[i+1] == '0'
}
