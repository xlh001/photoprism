package clean

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUri(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		result := Uri("https://docs.photoprism.app/getting-started/config-options/#file-converters")
		assert.Equal(t, "https://docs.photoprism.app/getting-started/config-options/#file-converters", result)
	})
	t.Run("Invalid", func(t *testing.T) {
		result := Uri("https://..docs.photoprism.app/gettin\\g-started/config-options/\tfile-converters")
		assert.Equal(t, "", result)
	})
	t.Run("Emoji", func(t *testing.T) {
		result := Uri("Hello 👍")
		assert.Equal(t, "Hello%20%F0%9F%91%8D", result)
	})
	t.Run("Empty", func(t *testing.T) {
		result := Uri("")
		assert.Equal(t, "", result)
	})
}

func TestUriRedacted(t *testing.T) {
	t.Run("WithCredentials", func(t *testing.T) {
		result := UriRedacted("https://user:secret@example.com/path?q=1")
		assert.Equal(t, "https://user:xxxxx@example.com/path?q=1", result)
	})
	t.Run("WithoutCredentials", func(t *testing.T) {
		result := UriRedacted("https://docs.photoprism.app/getting-started/config-options/#file-converters")
		assert.Equal(t, "https://docs.photoprism.app/getting-started/config-options/#file-converters", result)
	})
	t.Run("Invalid", func(t *testing.T) {
		result := UriRedacted("https://..docs.photoprism.app/gettin\\g-started/config-options/\tfile-converters")
		assert.Equal(t, "", result)
	})
	t.Run("Empty", func(t *testing.T) {
		result := UriRedacted("")
		assert.Equal(t, "", result)
	})
}

func BenchmarkUri(b *testing.B) {
	for b.Loop() {
		Uri("https://docs.photoprism.app/getting-started/config-options/#file-converters")
	}
}

func BenchmarkUriRedacted(b *testing.B) {
	for b.Loop() {
		UriRedacted("https://user:secret@docs.photoprism.app/getting-started/config-options/#file-converters")
	}
}

func BenchmarkUriEmpty(b *testing.B) {
	for b.Loop() {
		Uri("")
	}
}

func TestUriCredentialParam(t *testing.T) {
	t.Run("Credential", func(t *testing.T) {
		for _, name := range []string{
			"key", "api_key", "X-Api-Key", "apikey", "token", "access_token", "AccessToken",
			"secret", "client_secret", "password", "passwd", "pwd", "auth", "authorization",
			"credential", "credentials", "sig", "signature",
		} {
			assert.Truef(t, UriCredentialParam(name), "%s must be treated as a credential", name)
		}
	})
	t.Run("NotACredential", func(t *testing.T) {
		for _, name := range []string{"tier", "model", "format", "stream", "temperature", "n", ""} {
			assert.Falsef(t, UriCredentialParam(name), "%s must be shown", name)
		}
	})
}

func TestUriRedactedQuery(t *testing.T) {
	t.Run("Unparsable", func(t *testing.T) {
		assert.Equal(t, "", UriRedacted("://nope"))
	})
	t.Run("NoQuery", func(t *testing.T) {
		assert.Equal(t, "https://api.example.com/v1", UriRedacted("https://api.example.com/v1"))
	})
	t.Run("QueryOrderIsNotRelevant", func(t *testing.T) {
		// Re-encoding sorts the parameters, so the assertion is on what each one holds.
		result := UriRedacted("https://api.example.com/v1?z=1&api_key=notreal&a=2")
		assert.Contains(t, result, "api_key=xxxxx")
		assert.Contains(t, result, "z=1")
		assert.Contains(t, result, "a=2")
		assert.NotContains(t, result, "notreal")
	})
	t.Run("UserinfoAndQuery", func(t *testing.T) {
		// A value that reads as redacted must not sit beside one that is not.
		result := UriRedacted("https://user:pass@api.example.com/v1?access_token=notreal")
		assert.NotContains(t, result, "pass@")
		assert.NotContains(t, result, "notreal")
		assert.Contains(t, result, "user:xxxxx@")
		assert.Contains(t, result, "access_token=xxxxx")
	})
}
