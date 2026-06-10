package webtest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

// Client drives a wired Echo app (see Server) through full round trips,
// carrying cookies between requests so session flows test end to end. It
// sends Sec-Fetch-Site: same-origin by default so POSTs pass the
// same-origin gate; override the header to test the gate itself.
type Client struct {
	t       *testing.T
	e       *echo.Echo
	cookies map[string]*http.Cookie
}

func NewClient(t *testing.T, e *echo.Echo) *Client {
	t.Helper()
	return &Client{t: t, e: e, cookies: make(map[string]*http.Cookie)}
}

func (cl *Client) Get(path string) *httptest.ResponseRecorder {
	cl.t.Helper()
	return cl.Do(http.MethodGet, path, nil, nil)
}

func (cl *Client) PostForm(path string, form url.Values) *httptest.ResponseRecorder {
	cl.t.Helper()
	return cl.Do(http.MethodPost, path, form, nil)
}

// Do performs one request. headers (may be nil) are set after the defaults,
// so tests can override Sec-Fetch-Site or unset it with an empty value.
func (cl *Client) Do(method, path string, form url.Values, headers map[string]string) *httptest.ResponseRecorder {
	cl.t.Helper()

	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}

	req := httptest.NewRequestWithContext(cl.t.Context(), method, path, body)
	if form != nil {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	}
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	for k, v := range headers {
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	for _, c := range cl.cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	cl.e.ServeHTTP(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	for _, c := range res.Cookies() {
		if c.MaxAge < 0 {
			delete(cl.cookies, c.Name)
		} else {
			cl.cookies[c.Name] = c
		}
	}

	return rec
}
