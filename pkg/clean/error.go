package clean

import (
	iofs "io/fs"
	"os"
	"reflect"
	"sort"
	"strings"
)

const (
	// errorPathPlaceholder replaces the file paths removed by Error.
	errorPathPlaceholder = "***"
	// errorPathSeparators are the characters a value must contain to count as a location.
	errorPathSeparators = `/\`
	// errorPathTrivial are the characters a location must consist of more than.
	errorPathTrivial = `/\.`
	// errorUnwrapNodes bounds how many errors in a chain are inspected.
	errorUnwrapNodes = 256
	// errorOmitted replaces a message whose chain is too large to inspect completely.
	errorOmitted = "error details omitted"
)

// Error sanitizes an error message so that it can be safely logged or displayed, replacing the
// file paths the errors in its chain carry with a placeholder. A path that a wrapper rendered
// into message text stays, so producers wrap with %w. Use ErrorFull where the reader is an operator.
func Error(err error) string {
	if err == nil {
		return "no error"
	}

	paths, complete := errorPaths(err)

	// A chain too large to inspect completely may name a path this did not collect, so the
	// message is dropped rather than rendered as though it had been. The caller's own text
	// still identifies the site, and the oversized message is never materialized.
	if !complete {
		return errorOmitted
	}

	s := err.Error()

	for _, p := range paths {
		s = strings.ReplaceAll(s, p, errorPathPlaceholder)
	}

	return errorText(s)
}

// ErrorFull sanitizes an error message and keeps the file paths it names. Reserve it for the
// console and the CLI, where the path is the detail an operator acts on.
func ErrorFull(err error) string {
	if err == nil {
		return "no error"
	}

	return errorText(err.Error())
}

// errorText limits the length of an error message and removes problematic characters.
func errorText(s string) string {
	if s = strings.TrimSpace(s); s == "" {
		return "unknown error"
	}

	// Limit error message length.
	if len(s) > LengthLimit {
		s = s[:LengthLimit]
	}

	// Remove non-printable and other potentially problematic characters.
	s = strings.Map(func(r rune) rune {
		switch {
		case unsafeSpaceRune(r):
			return unsafeSpace
		case unsafeDropRune(r):
			return -1
		case unsafeRune(r):
			return unsafeMarker
		}

		switch r {
		case '`', '"':
			return '\''
		case '%', '\\', '$', '<', '>', '{', '}':
			return '?'
		default:
			return r
		}
	}, s)

	// A message of only dropped characters empties here, leaving a failure with no cause.
	if strings.TrimSpace(s) == "" {
		return "unknown error"
	}

	return s
}

// errorLocation reports whether a value names a location rather than a bare name. Replacement
// is by substring, so a value without a separator would also match ordinary words, and one
// built only from separators and dots would match a path fragment of every message.
func errorLocation(s string) bool {
	return strings.ContainsAny(s, errorPathSeparators) && strings.Trim(s, errorPathTrivial) != ""
}

// errorPaths returns the file paths named by err and the errors it wraps, deduplicated and
// longest first so that replacing one cannot leave a shorter path's remainder behind. It
// reports complete unless the chain exceeded the node budget, which also ends a cycle.
func errorPaths(err error) (out []string, complete bool) {
	seen := make(map[string]struct{})
	nodes := 0
	complete = true

	var walk func(error)

	walk = func(e error) {
		if e == nil {
			return
		} else if nodes >= errorUnwrapNodes {
			complete = false
			return
		}

		// A non-nil interface can hold a nil pointer, which neither branch below survives.
		if v := reflect.ValueOf(e); v.Kind() == reflect.Pointer && v.IsNil() {
			return
		}

		nodes++

		var found []string

		switch t := e.(type) {
		case *iofs.PathError:
			found = []string{t.Path}
		case *os.LinkError:
			found = []string{t.Old, t.New}
		}

		for _, p := range found {
			if _, dup := seen[p]; !dup && errorLocation(p) {
				seen[p] = struct{}{}
				out = append(out, p)
			}
		}

		switch u := e.(type) {
		case interface{ Unwrap() error }:
			walk(u.Unwrap())
		case interface{ Unwrap() []error }:
			for _, w := range u.Unwrap() {
				walk(w)
			}
		}
	}

	walk(err)

	sort.SliceStable(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })

	return out, complete
}
