package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVisionEndpoint(t *testing.T) {
	cases := []struct {
		name   string
		uri    string
		method string
		want   string
	}{
		{
			name:   "Plain",
			uri:    "http://ollama:11434/api/generate",
			method: "POST",
			want:   "POST http://ollama:11434/api/generate",
		},
		{ //nolint:gosec // example URL, the password in it is exactly what this case redacts
			name:   "BasicAuth",
			uri:    "https://vision:secret@vision.example.com/api/generate",
			method: "POST",
			want:   "POST https://vision:xxxxx@vision.example.com/api/generate",
		},
		{
			name:   "UsernameOnly",
			uri:    "https://vision@vision.example.com/api/generate",
			method: "POST",
			want:   "POST https://vision@vision.example.com/api/generate",
		},
		{
			name:   "QueryIsKept",
			uri:    "https://api.example.com/v1/responses?tier=flex",
			method: "POST",
			want:   "POST https://api.example.com/v1/responses?tier=flex",
		},
		{
			name:   "MissingUri",
			uri:    "",
			method: "POST",
			want:   "",
		},
		{
			name:   "MissingMethod",
			uri:    "https://api.example.com/v1/responses",
			method: "",
			want:   "",
		},
		{
			name:   "Unparsable",
			uri:    "://nope",
			method: "POST",
			want:   "POST ?",
		},
		{
			name:   "ApiKeyQuery",
			uri:    "https://api.example.com/v1/responses?api_key=notreal",
			method: "POST",
			want:   "POST https://api.example.com/v1/responses?api_key=xxxxx",
		},
		{
			name:   "AccessTokenQuery",
			uri:    "https://api.example.com/v1/responses?access_token=notreal",
			method: "POST",
			want:   "POST https://api.example.com/v1/responses?access_token=xxxxx",
		},
		{
			name:   "MixedCaseKeyQuery",
			uri:    "https://api.example.com/v1/responses?X-Api-Key=notreal",
			method: "POST",
			want:   "POST https://api.example.com/v1/responses?X-Api-Key=xxxxx",
		},
		{
			name:   "CredentialQueryBesideAKeptOne",
			uri:    "https://api.example.com/v1/responses?tier=flex&token=notreal",
			method: "POST",
			want:   "POST https://api.example.com/v1/responses?tier=flex&token=xxxxx",
		},
		{
			name:   "RepeatedCredentialQuery",
			uri:    "https://api.example.com/v1/responses?secret=one&secret=two",
			method: "POST",
			want:   "POST https://api.example.com/v1/responses?secret=xxxxx&secret=xxxxx",
		},
		{ //nolint:gosec // example URL, the credentials in it are exactly what this case redacts
			name:   "UserinfoAndQueryTogether",
			uri:    "https://vision:notreal@api.example.com/v1?signature=notreal",
			method: "POST",
			want:   "POST https://vision:xxxxx@api.example.com/v1?signature=xxxxx",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, visionEndpoint(tc.uri, tc.method))
		})
	}
}

func TestCredentialParam(t *testing.T) {
	t.Run("Credential", func(t *testing.T) {
		for _, name := range []string{
			"key", "api_key", "X-Api-Key", "apikey", "token", "access_token", "AccessToken",
			"secret", "client_secret", "password", "passwd", "pwd", "auth", "authorization",
			"credential", "credentials", "sig", "signature",
		} {
			assert.Truef(t, credentialParam(name), "%s must be treated as a credential", name)
		}
	})
	t.Run("NotACredential", func(t *testing.T) {
		for _, name := range []string{"tier", "model", "format", "stream", "temperature", "n", ""} {
			assert.Falsef(t, credentialParam(name), "%s must be shown", name)
		}
	})
}

func TestEndpointUriRedacted(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		assert.Equal(t, "", endpointUriRedacted(""))
		assert.Equal(t, "", endpointUriRedacted("   "))
	})
	t.Run("Unparsable", func(t *testing.T) {
		assert.Equal(t, "", endpointUriRedacted("://nope"))
	})
	t.Run("NoQuery", func(t *testing.T) {
		assert.Equal(t, "https://api.example.com/v1", endpointUriRedacted("https://api.example.com/v1"))
	})
	t.Run("QueryOrderIsNotRelevant", func(t *testing.T) {
		// Re-encoding sorts the parameters, so the assertion is on what each one holds.
		result := endpointUriRedacted("https://api.example.com/v1?z=1&api_key=notreal&a=2")
		assert.Contains(t, result, "api_key=xxxxx")
		assert.Contains(t, result, "z=1")
		assert.Contains(t, result, "a=2")
		assert.NotContains(t, result, "notreal")
	})
}
