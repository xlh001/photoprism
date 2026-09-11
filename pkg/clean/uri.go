package clean

import (
	"net/url"
	"strings"
)

// UriRedactedValue replaces a credential removed from a URI. It matches what net/url writes for a
// password, so a redacted query parameter reads like a redacted userinfo.
const UriRedactedValue = "xxxxx"

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
