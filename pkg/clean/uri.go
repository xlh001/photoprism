package clean

import (
	"net/url"
	"regexp"
	"strings"
)

// UriRedactedValue replaces a credential removed from a URI. It matches what net/url writes for a
// password, so a redacted query parameter reads like a redacted userinfo.
const UriRedactedValue = "xxxxx"

// uriUserinfo matches the userinfo of a URL wherever one appears in a longer text. The class
// excludes the characters RFC 3986 keeps out of a URI, so a compact JSON or key=value list that
// happens to hold a scheme and an at sign is not treated as one.
var uriUserinfo = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.\-]*://)([^\s/?#"'<>\\{}|^` + "`" + `]*)@`)

// UriCredentials replaces the credentials of every URL a text contains. It matches on the text
// rather than on a known value, so it applies wherever a URL was rendered into a message.
//
// A password is the secret and the name beside it is not, so that name is kept. A userinfo with no
// password cannot be told apart from a token, so all of it goes.
func UriCredentials(s string) string {
	if !strings.Contains(s, "://") {
		return s
	}

	return uriUserinfo.ReplaceAllStringFunc(s, func(match string) string {
		m := uriUserinfo.FindStringSubmatch(match)

		if user, password, found := strings.Cut(m[2], ":"); found && password != "" {
			return m[1] + user + ":" + UriRedactedValue + "@"
		}

		return m[1] + UriRedactedValue + "@"
	})
}

// uriCredentialParams are the query parameter names whose value is treated as a credential.
// Matched as substrings of the lowercased name, so "X-Api-Key" and "access_token" are covered.
var uriCredentialParams = []string{"key", "token", "secret", "password", "passwd", "pwd", "auth", "credential", "sig", "signature"}

// Uri removes invalid character from an uri string.
func Uri(s string) string {
	if s == "" || len(s) > LengthLimit {
		return ""
	} else if strings.Contains(s, "..") {
		return ""
	}

	// Trim whitespace.
	s = strings.TrimSpace(s)

	if uri, err := url.Parse(s); err != nil {
		return ""
	} else {
		return uri.String()
	}
}

// UriRedacted removes the credentials a URI carries, in the userinfo and in a query parameter,
// while preserving its other components. A service endpoint commonly authenticates through a
// query parameter rather than through the userinfo, so removing only the latter would leave a
// value that reads as redacted next to one that is not.
func UriRedacted(s string) string {
	if s == "" || len(s) > LengthLimit {
		return ""
	}

	// Trim whitespace.
	s = strings.TrimSpace(s)

	uri, err := url.Parse(s)

	if err != nil {
		return ""
	}

	// A userinfo with no password cannot be told apart from a token, so all of it goes; Redacted
	// below replaces a password and keeps the name beside it.
	if uri.User != nil {
		if password, set := uri.User.Password(); !set || password == "" {
			uri.User = url.User(UriRedactedValue)
		}
	}

	if q := uri.Query(); len(q) > 0 {
		redacted := false

		for name, values := range q {
			if !UriCredentialParam(name) {
				continue
			}

			for i := range values {
				values[i] = UriRedactedValue
			}

			q[name] = values
			redacted = true
		}

		if redacted {
			uri.RawQuery = q.Encode()
		}
	}

	return uri.Redacted()
}

// UriCredentialParam reports whether a query parameter name is one whose value must not be shown.
func UriCredentialParam(name string) bool {
	name = strings.ToLower(name)

	for _, s := range uriCredentialParams {
		if strings.Contains(name, s) {
			return true
		}
	}

	return false
}
