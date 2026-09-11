package clean

import (
	"fmt"
	"strings"

	"github.com/photoprism/photoprism/pkg/txt/clip"
)

const (
	// LogNamesLimit is the number of names LogNames renders before it counts the rest.
	LogNamesLimit = 10
	// LogNamesBytes is the budget LogNames gives each name it renders.
	LogNamesBytes = 128
)

// LogNames sanitizes a list of names for logging and counts those beyond LogNamesLimit, so that
// the length of the message follows the two limits and not the length of the list. Each name is
// bounded in bytes rather than characters, which keeps a multi-byte name inside the same budget.
func LogNames(names []string) string {
	if len(names) == 0 {
		return "''"
	}

	kept := min(len(names), LogNamesLimit)
	out := make([]string, 0, kept)

	for _, name := range names[:kept] {
		out = append(out, Log(clip.Bytes(name, LogNamesBytes)))
	}

	if omitted := len(names) - kept; omitted > 0 {
		return fmt.Sprintf("%s and %d more", strings.Join(out, ", "), omitted)
	}

	return strings.Join(out, ", ")
}

// Log sanitizes strings created from user input in response to the log4j debacle.
func Log(s string) string {
	if s == "" {
		return "''"
	}

	s = clip.Shorten(s, LengthLog, clip.Ellipsis)

	if reject(s, LengthLimit) {
		return "?"
	}

	spaces := false

	// Remove non-printable and other potentially problematic characters.
	s = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}

		switch r {
		case ' ':
			spaces = true
			return r
		case '`':
			return '\''
		case '"':
			return '"'
		case '\\', '$', '<', '>', '{', '}':
			return '?'
		default:
			return r
		}
	}, s)

	// Contains spaces?
	if spaces {
		return fmt.Sprintf("'%s'", s)
	}

	return s
}

// LogQuote sanitizes a string and puts it in single quotes for logging.
func LogQuote(s string) string {
	if s = Log(s); s[0] != '\'' {
		return fmt.Sprintf("'%s'", s)
	} else {
		return s
	}
}

// LogLower sanitizes strings created from user input and converts them to lowercase.
func LogLower(s string) string {
	return Log(strings.ToLower(s))
}
