package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"

	"github.com/photoprism/photoprism/internal/config"
	"github.com/photoprism/photoprism/internal/server/limiter"
	"github.com/photoprism/photoprism/pkg/authn"
	"github.com/photoprism/photoprism/pkg/http/header"
)

// countingBody reports how many bytes a handler read, so a test can assert that an
// oversized request is not consumed in full.
type countingBody struct {
	r    io.Reader
	read int64
}

func (b *countingBody) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	atomic.AddInt64(&b.read, int64(n))

	return n, err
}

// oversizedJSON returns a syntactically valid JSON object of roughly size bytes.
func oversizedJSON(size int) io.Reader {
	return io.MultiReader(
		strings.NewReader(`{"padding":"`),
		strings.NewReader(strings.Repeat("a", size)),
		strings.NewReader(`"}`),
	)
}

// postCounted sends a request with a counting body and returns the status and bytes read.
func postCounted(app http.Handler, path, contentType string, body io.Reader) (int, int64) {
	b := &countingBody{r: body}
	req := httptest.NewRequest(http.MethodPost, path, b)
	req.Header.Set(header.ContentType, contentType)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	return w.Code, atomic.LoadInt64(&b.read)
}

func TestOAuthRequestLimits(t *testing.T) {
	app, router, conf := NewApiTest()
	conf.SetAuthMode(config.AuthModePasswd)
	defer conf.SetAuthMode(config.AuthModePublic)
	OAuthToken(router)
	OAuthRevoke(router)

	// Well above MaxOAuthRequestBytes, and above the size Go's own form parser accepts.
	const oversized = 16 << 20

	t.Run("TokenJSON", func(t *testing.T) {
		code, read := postCounted(app, "/api/v1/oauth/token", header.ContentTypeJson, oversizedJSON(oversized))
		assert.Equal(t, http.StatusRequestEntityTooLarge, code)
		assert.LessOrEqual(t, read, MaxOAuthRequestBytes+1)
	})
	t.Run("TokenForm", func(t *testing.T) {
		// The grant_type peek parses the form, so the bound has to apply before it.
		body := io.MultiReader(strings.NewReader("grant_type="), strings.NewReader(strings.Repeat("a", oversized)))
		code, read := postCounted(app, "/api/v1/oauth/token", header.ContentTypeForm, body)
		assert.Equal(t, http.StatusRequestEntityTooLarge, code)
		assert.LessOrEqual(t, read, MaxOAuthRequestBytes+1)
	})
	t.Run("RevokeJSON", func(t *testing.T) {
		code, read := postCounted(app, "/api/v1/oauth/revoke", header.ContentTypeJson, oversizedJSON(oversized))
		assert.Equal(t, http.StatusRequestEntityTooLarge, code)
		assert.LessOrEqual(t, read, MaxOAuthRequestBytes+1)
	})
	t.Run("WithinLimitStillWorks", func(t *testing.T) {
		data := "grant_type=" + authn.GrantClientCredentials.String() + "&client_id=cs5cpu17n6gj2qo5&client_secret=xcCbOrw6I0vcoXzhnOmXhjpVSyFq0l0e&scope=metrics"
		code, _ := postCounted(app, "/api/v1/oauth/token", header.ContentTypeForm, strings.NewReader(data))
		assert.Equal(t, http.StatusOK, code)
	})
}

// TestOAuthToken_RateLimitChargesMalformedRequests covers that a request which never reaches
// a grant is still charged, so repeating it cannot stay free.
func TestOAuthToken_RateLimitChargesMalformedRequests(t *testing.T) {
	app, router, conf := NewApiTest()
	conf.SetAuthMode(config.AuthModePasswd)
	defer conf.SetAuthMode(config.AuthModePublic)
	OAuthToken(router)

	oldLogin, oldAuth := limiter.Login, limiter.Auth
	defer func() { limiter.Login, limiter.Auth = oldLogin, oldAuth }()
	limiter.Login = limiter.NewLimit(rate.Every(24*time.Hour), 3)
	limiter.Auth = limiter.NewLimit(rate.Every(24*time.Hour), 3)

	// A body that binds but carries no usable grant is rejected before any credential
	// lookup, and must still consume the failure budget.
	body := `{"padding":"nothing that matches a form field"}`

	for i := range 3 {
		code, _ := postCounted(app, "/api/v1/oauth/token", header.ContentTypeJson, strings.NewReader(body))
		assert.Equal(t, http.StatusUnauthorized, code, "request %d", i+1)
	}

	code, _ := postCounted(app, "/api/v1/oauth/token", header.ContentTypeJson, strings.NewReader(body))
	assert.Equal(t, http.StatusTooManyRequests, code)
}
