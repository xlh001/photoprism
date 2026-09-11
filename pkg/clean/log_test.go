package clean

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/photoprism/photoprism/pkg/txt/clip"
)

func TestLog(t *testing.T) {
	t.Run("TheQuickBrownFox", func(t *testing.T) {
		assert.Equal(t, "'The quick brown fox.'", Log("The quick brown fox."))
	})
	t.Run("FilenameTxt", func(t *testing.T) {
		assert.Equal(t, "filename.txt", Log("filename.txt"))
	})
	t.Run("EmptyString", func(t *testing.T) {
		assert.Equal(t, "''", Log(""))
	})
	t.Run("Replace", func(t *testing.T) {
		assert.Equal(t, "?", Log("${https://<host>:<port>/<path>}"))
	})
	t.Run("Ldap", func(t *testing.T) {
		assert.Equal(t, "?", Log("User-Agent: {jndi:ldap://<host>:<port>/<path>}"))
	})
	t.Run("SpecialChars", func(t *testing.T) {
		assert.Equal(t, "'  The ?quick? ''brown \"fox.   '", Log("  The <quick>\n\r ''brown \"fox. \t  "))
	})
	t.Run("LoremIpsum", func(t *testing.T) {
		assert.Equal(t, "'It is a long established fact that a reader will be distracted by the readable "+
			"content of a pagewhen looking at its layout. The point of using Lorem Ipsum is that it has a "+
			"more-or-less normal distribution of letters,as opposed to using 'Content here, content here', making it "+
			"look like readable English.Many desktop publishing packages and web page editors now use Lorem Ipsum as "+
			"their default model text, and a search for'lorem ipsum' will uncover many web sites still in their "+
			"infancy. Various versions…'", Log(clip.LoremIpsum))
	})
}

func TestLogQuote(t *testing.T) {
	t.Run("TheQuickBrownFox", func(t *testing.T) {
		assert.Equal(t, "'The quick brown fox.'", LogQuote("The quick brown fox."))
	})
	t.Run("SpecialChars", func(t *testing.T) {
		assert.Equal(t, "'?The quick brown fox'", LogQuote("$The quick brown fox"))
	})
}

func TestLogLower(t *testing.T) {
	t.Run("TheQuickBrownFox", func(t *testing.T) {
		assert.Equal(t, "'the quick brown fox.'", LogLower("The quick brown fox."))
	})
	t.Run("FilenameTxt", func(t *testing.T) {
		assert.Equal(t, "filename.txt", LogLower("filename.TXT"))
	})
	t.Run("EmptyString", func(t *testing.T) {
		assert.Equal(t, "''", LogLower(""))
	})
	t.Run("Replace", func(t *testing.T) {
		assert.Equal(t, "?", LogLower("${https://<host>:<port>/<path>}"))
	})
	t.Run("Ldap", func(t *testing.T) {
		assert.Equal(t, "?", LogLower("User-Agent: ${jndi:ldap://<host>:<port>/<path>}"))
	})
}

func TestLogNames(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		assert.Equal(t, "''", LogNames(nil))
		assert.Equal(t, "''", LogNames([]string{}))
	})
	t.Run("One", func(t *testing.T) {
		assert.Equal(t, "filename.txt", LogNames([]string{"filename.txt"}))
	})
	t.Run("Several", func(t *testing.T) {
		assert.Equal(t, "a.jpg, b.jpg, c.jpg", LogNames([]string{"a.jpg", "b.jpg", "c.jpg"}))
	})
	t.Run("AtLimit", func(t *testing.T) {
		names := make([]string, LogNamesLimit)
		for i := range names {
			names[i] = "a.jpg"
		}
		result := LogNames(names)
		assert.NotContains(t, result, "more")
		assert.Equal(t, LogNamesLimit, strings.Count(result, "a.jpg"))
	})
	t.Run("BeyondLimitIsCounted", func(t *testing.T) {
		names := make([]string, LogNamesLimit+5)
		for i := range names {
			names[i] = "a.jpg"
		}
		result := LogNames(names)
		assert.Equal(t, LogNamesLimit, strings.Count(result, "a.jpg"))
		assert.Contains(t, result, "and 5 more")
	})
	t.Run("LengthFollowsTheLimits", func(t *testing.T) {
		// The rendered length must follow the two limits, not the length of the list or of a name.
		bound := LogNamesLimit*(LogNamesBytes+8) + 32
		for _, fill := range []string{"a", "\u00e4", "\u4e2d", "\U0001F600"} {
			names := make([]string, 10000)
			for i := range names {
				names[i] = strings.Repeat(fill, 600) + ".jpg"
			}
			assert.Less(t, len(LogNames(names)), bound, "fill %q", fill)
		}
	})
	t.Run("NameIsBoundedInBytes", func(t *testing.T) {
		// A multi-byte name gets the same budget as an ASCII one.
		for _, fill := range []string{"a", "\u4e2d", "\U0001F600"} {
			assert.LessOrEqual(t, len(LogNames([]string{strings.Repeat(fill, 600)})), LogNamesBytes+8, "fill %q", fill)
		}
	})
	t.Run("EachNameIsSanitized", func(t *testing.T) {
		result := LogNames([]string{"ok.jpg", "report\nsummary.jpg", "x\ry.jpg"})
		assert.NotContains(t, result, "\n")
		assert.NotContains(t, result, "\r")
		assert.Contains(t, result, "ok.jpg")
	})
	t.Run("InjectionIsRejected", func(t *testing.T) {
		assert.Equal(t, "?", LogNames([]string{"${jndi:ldap://host:1389/a}"}))
	})
}
