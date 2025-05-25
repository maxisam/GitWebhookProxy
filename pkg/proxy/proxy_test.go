package proxy

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	httpmock "github.com/jarcoal/httpmock"
	"github.com/julienschmidt/httprouter"
	"github.com/stakater/GitWebhookProxy/pkg/providers"
)

const (
	proxyGitlabTestSecret = "testSecret"
	proxyGitlabTestEvent  = "Push Hook"
	proxyGitlabTestBody   = "testBody"
	httpBinURL            = "httpbin.org"
	httpBinURLInsecure    = "http://" + httpBinURL
	httpBinURLSecure      = "https://" + httpBinURL
)

var (
	proxyGitlabTestPayload = getGitlabPayload()
)

func getGitlabPayload() []byte {
	payload, _ := ioutil.ReadFile("gitlab_test_payload.json")
	return payload
}

func TestProxy_isPathAllowed(t *testing.T) {
	type fields struct {
		provider     string
		upstreamURL  string
		allowedPaths []string
		secret       string
	}
	type args struct {
		path string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "isPathAllowedWithValidMultipleAllowedPaths",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2"},
				secret:       "secret",
			},
			args: args{
				path: "/path2",
			},
			want: true,
		},
		{
			name: "isPathAllowedWithValidOneAllowedPaths",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1"},
				secret:       "secret",
			},
			args: args{
				path: "/path1",
			},
			want: true,
		},
		{
			name: "isPathAllowedWithInvalidPath",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2"},
				secret:       "secret",
			},
			args: args{
				path: "/path3",
			},
			want: false,
		},
		{
			name: "isPathAllowedWithEmtpyPathArg",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2"},
				secret:       "secret",
			},
			args: args{
				path: "",
			},
			want: false,
		},
		{
			name: "isPathAllowedWithAllPathsAllowedAndEmptyPathArg",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{},
				secret:       "secret",
			},
			args: args{
				path: "",
			},
			want: true,
		},
		{
			name: "isPathAllowedWithAllPathsAllowedAndRootEmptyPathArg",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{},
				secret:       "secret",
			},
			args: args{
				path: "/",
			},
			want: true,
		},
		{
			name: "isPathAllowedWithAllPathsAllowedAndNonEmptyPathArg",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{},
				secret:       "secret",
			},
			args: args{
				path: "/path1",
			},
			want: true,
		},
		{
			name: "isPathAllowedWithSomePathsAllowedAndRootPathArg",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2"},
				secret:       "secret",
			},
			args: args{
				path: "/",
			},
			want: false,
		},
		{
			name: "isPathAllowedWithSomePathsAllowedAndSubPathArg",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path4"},
				secret:       "secret",
			},
			args: args{
				path: "/path2/path3",
			},
			want: false,
		},
		{
			name: "isPathAllowedWithSubPathsAllowedAndSubPathArg",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2/path3"},
				secret:       "secret",
			},
			args: args{
				path: "/path2/path3",
			},
			want: true,
		},
		{
			name: "isPathAllowedWithSubPathsAllowedAndPathArg",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2/path3"},
				secret:       "secret",
			},
			args: args{
				path: "/path2",
			},
			want: false,
		},
		{
			name: "isPathAllowedWithAllowedPathTrailingSlashAndNotInArg",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2/"},
				secret:       "secret",
			},
			args: args{
				path: "/path2",
			},
			want: true,
		},
		{
			name: "isPathAllowedWithSimpleAllowedPathAndTrailingSlashInArg",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2"},
				secret:       "secret",
			},
			args: args{
				path: "/path2/",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Proxy{
				provider:     tt.fields.provider,
				upstreamURL:  tt.fields.upstreamURL,
				allowedPaths: tt.fields.allowedPaths,
				secret:       tt.fields.secret,
			}
			if got := p.isPathAllowed(tt.args.path); got != tt.want {
				t.Errorf("Proxy.isPathAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func createGitlabHook(tokenHeader string, tokenEvent string, body string, method string) *providers.Hook {
	return &providers.Hook{
		Headers: map[string]string{
			providers.XGitlabToken: tokenHeader,
			providers.XGitlabEvent: tokenEvent,
		},
		Payload:       []byte(body),
		RequestMethod: method,
	}
}

func TestProxy_redirect(t *testing.T) {

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", httpBinURLSecure,
		httpmock.NewStringResponder(200, ``))

	httpmock.RegisterResponder("POST", httpBinURLSecure+"/get",
		httpmock.NewStringResponder(405, ``))

	httpmock.RegisterResponder("POST", httpBinURLSecure+"/post",
		httpmock.NewStringResponder(200, ``))

	httpmock.RegisterResponder("POST", httpBinURLInsecure+"/post",
		httpmock.NewStringResponder(200, ``))

	type fields struct {
		provider     string
		upstreamURL  string
		allowedPaths []string
		secret       string
	}
	type args struct {
		hook        *providers.Hook
		redirectURL string
	}
	tests := []struct {
		name               string
		fields             fields
		args               args
		wantStatusCode     int
		wantRedirectedHost string // Only Host not complete URL
		wantErr            bool
	}{
		{
			name: "TestRedirectWithValidValues",
			fields: fields{
				provider:     "gitlab",
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{},
				secret:       "dummy",
			},
			args: args{
				redirectURL: httpBinURLSecure + "/post",
				hook:        createGitlabHook(proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestBody, http.MethodPost),
			},
			wantStatusCode:     http.StatusOK,
			wantRedirectedHost: httpBinURL,
		},
		{
			name: "TestRedirectWithGetUpstream",
			fields: fields{
				provider:     "gitlab",
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{},
				secret:       "dummy",
			},
			args: args{
				redirectURL: httpBinURLSecure + "/get",
				hook:        createGitlabHook(proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestBody, http.MethodPost),
			},
			wantStatusCode:     http.StatusMethodNotAllowed,
			wantRedirectedHost: httpBinURL,
		},
		{
			name: "TestRedirectWithEmptyPath",
			fields: fields{
				provider:     "github",
				upstreamURL:  httpBinURLSecure + "/post",
				allowedPaths: []string{},
				secret:       "dummy",
			},
			args: args{
				redirectURL: httpBinURLSecure + "/post",
				hook:        createGitlabHook(proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestBody, http.MethodPost),
			},
			wantStatusCode:     http.StatusOK,
			wantRedirectedHost: httpBinURL,
		},
		{
			name: "TestRedirectWithEmptyPath",
			fields: fields{
				provider:     "github",
				upstreamURL:  httpBinURLSecure + "/post",
				allowedPaths: []string{},
				secret:       "dummy",
			},
			args: args{
				redirectURL: httpBinURLSecure + "/post",
				hook:        createGitlabHook(proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestBody, http.MethodPost),
			},
			wantStatusCode:     http.StatusOK,
			wantRedirectedHost: httpBinURL,
		},
		{
			name: "TestRedirectWithNilHook",
			fields: fields{
				provider:     "github",
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{},
				secret:       "dummy",
			},
			args: args{
				redirectURL: httpBinURLSecure + "/post",
				hook:        nil,
			},
			wantErr: true,
		},
		{
			name: "TestRedirectWithInvalidUrl",
			fields: fields{
				provider:     "gitlab",
				upstreamURL:  "https://invalidurl",
				allowedPaths: []string{},
				secret:       "dummy",
			},
			args: args{
				redirectURL: "https://invalidurl/post",
				hook:        createGitlabHook(proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestBody, http.MethodPost),
			},
			wantErr: true,
		},
		{
			name: "TestRedirectWithInvalidUrlScheme",
			fields: fields{
				provider:     "gitlab",
				upstreamURL:  "htttpsss://" + httpBinURL,
				allowedPaths: []string{},
				secret:       "dummy",
			},
			args: args{
				redirectURL: "htttpsss://" + httpBinURL + "/post",
				hook:        createGitlabHook(proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestBody, http.MethodPost),
			},
			wantErr: true,
		},
		{
			name: "TestRedirectWithUrlWithoutScheme",
			fields: fields{
				provider:     "gitlab",
				upstreamURL:  httpBinURL,
				allowedPaths: []string{},
				secret:       "dummy",
			},
			args: args{
				redirectURL: httpBinURL + "/post",
				hook:        createGitlabHook(proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestBody, http.MethodPost),
			},
			wantStatusCode:     http.StatusOK,
			wantRedirectedHost: httpBinURL,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Proxy{
				provider:     tt.fields.provider,
				upstreamURL:  tt.fields.upstreamURL,
				allowedPaths: tt.fields.allowedPaths,
				secret:       tt.fields.secret,
			}
			gotResp, gotErrors := p.redirect(tt.args.hook, tt.args.redirectURL)

			if (gotErrors != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", gotErrors, tt.wantErr)
				return
			}

			if tt.wantErr == true && gotErrors != nil {
				return
			}

			if gotResp.StatusCode != tt.wantStatusCode {
				t.Errorf("Proxy.redirect() got StatusCode in response= %v, want %v",
					gotResp.StatusCode, tt.wantStatusCode)
				return
			}

			if gotResp.Request.Host != tt.wantRedirectedHost {
				t.Errorf("Proxy.redirect() got Redirected Host in response= %v, want Redirected Host= %v",
					gotResp.Request.Host, tt.wantRedirectedHost)
				return
			}

		})
	}
}

func createGitlabRequest(method string, path string, tokenHeader string,
	eventHeader string, body string) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Add(providers.XGitlabToken, tokenHeader)
	req.Header.Add(providers.XGitlabEvent, eventHeader)
	req.Header.Add(providers.ContentTypeHeader, providers.DefaultContentTypeHeaderValue)
	return req
}

func createGitlabRequestWithPayload(method string, path string, tokenHeader string,
	eventHeader string, body []byte) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Add(providers.XGitlabToken, tokenHeader)
	req.Header.Add(providers.XGitlabEvent, eventHeader)
	req.Header.Add(providers.ContentTypeHeader, providers.DefaultContentTypeHeaderValue)
	return req
}

func createRequestWithWrongHeadersKeys(method string, path string, tokenHeader string,
	eventHeader string, body string) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Add("X-Wrong-Token", tokenHeader)
	req.Header.Add("X-Wrong-Event", eventHeader)
	return req
}

func createRequestWithoutHeaders(method string, path string, body string) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	return req
}

func TestProxy_proxyRequest(t *testing.T) {
	type fields struct {
		provider     string
		upstreamURL  string
		allowedPaths []string
		secret       string
		allowedUsers []string
	}
	type args struct {
		request *http.Request
	}
	tests := []struct {
		name           string
		fields         fields
		args           args
		wantStatusCode int
	}{
		{
			name: "TestProxyRequestWithValidValues",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequestWithPayload(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestPayload),
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "TestProxyRequestWithoutConfiguringSecret",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{},
				secret:       "",
			},
			args: args{
				request: createGitlabRequestWithPayload(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestPayload),
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "TestProxyRequestWithoutSecretHearderInRequest",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequestWithPayload(http.MethodPost, "/post",
					"", proxyGitlabTestEvent, proxyGitlabTestPayload),
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "TestProxyRequestWithInvalidSecretInHeader",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					"InvalidSecret", proxyGitlabTestEvent, proxyGitlabTestBody),
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "TestProxyRequestWithEmptySecretInHeader",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					"", proxyGitlabTestEvent, proxyGitlabTestBody),
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "TestProxyRequestWithEmptyEventInHeader",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					proxyGitlabTestSecret, "", proxyGitlabTestBody),
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "TestProxyRequestWithWrongHeaderKeys",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createRequestWithWrongHeadersKeys(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestBody),
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "TestProxyRequestWithoutHeaderKeys",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createRequestWithoutHeaders(http.MethodPost, "/post", proxyGitlabTestBody),
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "TestProxyRequestWithUnsupportedUrlPath",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequestWithPayload(http.MethodPost, "/get",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestPayload),
			},
			wantStatusCode: http.StatusMethodNotAllowed,
		},
		{
			name: "TestProxyRequestShouldNotParseJsonWithoutAllowedOrIgnoredUsersConfigured",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{},
				secret:       "",
			},
			args: args{
				request: createGitlabRequestWithPayload(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, []byte("{}")),
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "TestProxyRequestShouldParseJsonWithAllowedOrIgnoredUsersConfigured",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{},
				secret:       "",
				allowedUsers: []string{"jsmith"},
			},
			args: args{
				request: createGitlabRequestWithPayload(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestPayload),
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "TestProxyRequestWithInvalidHttpMethod",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodGet, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestBody),
			},
			wantStatusCode: http.StatusMethodNotAllowed,
		},
		{
			name: "TestProxyRequestWithEmptyBody",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, ""),
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "TestProxyRequestWithNotAllowedPath",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{"/path1"},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestSecret),
			},
			wantStatusCode: http.StatusForbidden,
		},
		{
			name: "TestProxyRequestWithAllowedPath",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{"/post"},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestSecret),
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "TestProxyRequestWithInvalidUpstreamUrl",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  "invalidurl",
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestSecret),
			},
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name: "TestProxyRequestWithInvalidProvider",
			fields: fields{
				provider:     "invalid",
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestSecret),
			},
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name: "TestProxyRequestWithWrongProviderKind",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestSecret),
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "TestProxyRequestWithInvalidSecretInProvider",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{},
				secret:       "wrong",
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestSecret),
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "TestProxyRequestWithEmptySecretInProvider",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURLSecure,
				allowedPaths: []string{},
				secret:       "",
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestSecret),
			},
			wantStatusCode: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Proxy{
				provider:     tt.fields.provider,
				upstreamURL:  tt.fields.upstreamURL,
				allowedPaths: tt.fields.allowedPaths,
				secret:       tt.fields.secret,
				allowedUsers: tt.fields.allowedUsers,
			}
			router := httprouter.New()
			router.POST("/*path", p.proxyRequest)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, tt.args.request)

			if status := rr.Code; status != tt.wantStatusCode {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.wantStatusCode)
			}

		})
	}
}

func TestProxy_health(t *testing.T) {
	type fields struct {
		provider     string
		upstreamURL  string
		allowedPaths []string
		secret       string
	}
	type args struct {
		httpMethod string
	}
	tests := []struct {
		name           string
		fields         fields
		args           args
		wantStatusCode int
	}{
		{
			name: "TestHealthCheckGet",
			args: args{
				httpMethod: http.MethodGet,
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "TestHealthCheckPost",
			args: args{
				httpMethod: http.MethodPost,
			},
			wantStatusCode: http.StatusMethodNotAllowed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Proxy{
				provider:     tt.fields.provider,
				upstreamURL:  tt.fields.upstreamURL,
				allowedPaths: tt.fields.allowedPaths,
				secret:       tt.fields.secret,
			}
			router := httprouter.New()
			router.GET("/health", p.health)

			req, err := http.NewRequest(tt.args.httpMethod, "/health", nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.wantStatusCode {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.wantStatusCode)
			}
		})
	}
}

func TestProxy_Run(t *testing.T) {
	type fields struct {
		provider     string
		upstreamURL  string
		allowedPaths []string
		secret       string
	}
	type args struct {
		listenAddress string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		//https://stackoverflow.com/questions/46778600/golang-execute-function-after-http-listenandserve
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Proxy{
				provider:     tt.fields.provider,
				upstreamURL:  tt.fields.upstreamURL,
				allowedPaths: tt.fields.allowedPaths,
				secret:       tt.fields.secret,
			}
			if err := p.Run(tt.args.listenAddress); (err != nil) != tt.wantErr {
				t.Errorf("Proxy.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewProxy(t *testing.T) {
	type args struct {
		upstreamURLs []string
		allowedPaths []string
		provider     string
		secret       string
		ignoredUsers []string
	}
	tests := []struct {
		name    string
		args    args
		want    *Proxy
		wantErr bool
	}{
		{
			name: "TestNewProxyWithValidArgs",
			args: args{
				upstreamURLs: []string{httpBinURLSecure},
				allowedPaths: []string{},
				provider:     providers.GitlabProviderKind,
				secret:       proxyGitlabTestSecret,
			},
			want: &Proxy{
				upstreamURLs: []string{httpBinURLSecure},
				allowedPaths: []string{},
				provider:     providers.GitlabProviderKind,
				secret:       proxyGitlabTestSecret,
			},
		},
		{
			name: "TestNewProxyWithMultipleValidUpstreamURLs",
			args: args{
				upstreamURLs: []string{httpBinURLSecure, httpBinURLInsecure},
				allowedPaths: []string{},
				provider:     providers.GitlabProviderKind,
				secret:       proxyGitlabTestSecret,
			},
			want: &Proxy{
				upstreamURLs: []string{httpBinURLSecure, httpBinURLInsecure},
				allowedPaths: []string{},
				provider:     providers.GitlabProviderKind,
				secret:       proxyGitlabTestSecret,
			},
		},
		{
			name: "TestNewProxyWithEmptyUpstreamURLsSlice",
			args: args{
				upstreamURLs: []string{},
				allowedPaths: []string{},
				provider:     providers.GitlabProviderKind,
				secret:       proxyGitlabTestSecret,
			},
			wantErr: true, // Expects "Cannot create Proxy with no upstreamURLs"
		},
		{
			name: "TestNewProxyWithNilUpstreamURLsSlice",
			args: args{
				upstreamURLs: nil,
				allowedPaths: []string{},
				provider:     providers.GitlabProviderKind,
				secret:       proxyGitlabTestSecret,
			},
			wantErr: true, // Expects "Cannot create Proxy with no upstreamURLs"
		},
		{
			name: "TestNewProxyWithUpstreamURLsSliceContainingEmptyString",
			args: args{
				upstreamURLs: []string{""},
				allowedPaths: []string{},
				provider:     providers.GitlabProviderKind,
				secret:       proxyGitlabTestSecret,
			},
			wantErr: true, // Expects "Cannot create Proxy with an empty URL in upstreamURLs list"
		},
		{
			name: "TestNewProxyWithUpstreamURLsSliceContainingValidAndEmptyString",
			args: args{
				upstreamURLs: []string{httpBinURLSecure, ""},
				allowedPaths: []string{},
				provider:     providers.GitlabProviderKind,
				secret:       proxyGitlabTestSecret,
			},
			wantErr: true, // Expects "Cannot create Proxy with an empty URL in upstreamURLs list"
		},
		{
			name: "TestNewProxyWithDuplicateValidUpstreamURLs",
			args: args{
				upstreamURLs: []string{httpBinURLSecure, httpBinURLSecure},
				allowedPaths: []string{},
				provider:     providers.GitlabProviderKind,
				secret:       proxyGitlabTestSecret,
			},
			want: &Proxy{
				upstreamURLs: []string{httpBinURLSecure, httpBinURLSecure},
				allowedPaths: []string{},
				provider:     providers.GitlabProviderKind,
				secret:       proxyGitlabTestSecret,
			},
			wantErr: false,
		},
		{
			name: "TestNewProxyWithNilAllowedPaths",
			args: args{
				upstreamURLs: []string{httpBinURLSecure},
				allowedPaths: nil,
				provider:     providers.GitlabProviderKind,
				secret:       proxyGitlabTestSecret,
			},
			wantErr: true,
		},
		{
			name: "TestNewProxyWithEmptyProvider",
			args: args{
				upstreamURLs: []string{httpBinURLSecure},
				allowedPaths: []string{},
				provider:     "",
				secret:       proxyGitlabTestSecret,
			},
			wantErr: true,
		},
		{
			name: "TestNewProxyWithEmptySecretAndNilAllowedPaths", // secret can be empty, but allowedPaths nil is an error
			args: args{
				upstreamURLs: []string{httpBinURLSecure},
				allowedPaths: nil,
				provider:     providers.GitlabProviderKind,
				secret:       "",
			},
			wantErr: true,
		},
		{
			name: "TestNewProxyWithEmptySecretAndValidAllowedPaths", // secret can be empty
			args: args{
				upstreamURLs: []string{httpBinURLSecure},
				allowedPaths: []string{},
				provider:     providers.GitlabProviderKind,
				secret:       "",
			},
			want: &Proxy{
				upstreamURLs: []string{httpBinURLSecure},
				allowedPaths: []string{},
				provider:     providers.GitlabProviderKind,
				secret:       "",
			},
		},
		{
			name: "TestNewProxyWithValidArgsAndAllowedPaths",
			args: args{
				upstreamURLs: []string{httpBinURLSecure},
				allowedPaths: []string{"/path1", "/path2"},
				provider:     providers.GitlabProviderKind,
				secret:       proxyGitlabTestSecret,
				ignoredUsers: []string{"user1"},
			},
			want: &Proxy{
				upstreamURLs: []string{httpBinURLSecure},
				allowedPaths: []string{"/path1", "/path2"},
				provider:     providers.GitlabProviderKind,
				secret:       proxyGitlabTestSecret,
				ignoredUsers: []string{"user1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewProxy(tt.args.upstreamURLs, tt.args.allowedPaths, tt.args.provider, tt.args.secret, tt.args.ignoredUsers)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProxy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				expectedErrorMsg := ""
				if tt.name == "TestNewProxyWithEmptyUpstreamURLsSlice" || tt.name == "TestNewProxyWithNilUpstreamURLsSlice" {
					expectedErrorMsg = "Cannot create Proxy with no upstreamURLs"
				} else if tt.name == "TestNewProxyWithUpstreamURLsSliceContainingEmptyString" || tt.name == "TestNewProxyWithUpstreamURLsSliceContainingValidAndEmptyString" {
					expectedErrorMsg = "Cannot create Proxy with an empty URL in upstreamURLs list"
				}
				if err != nil && err.Error() != expectedErrorMsg {
					t.Errorf("NewProxy() error = %v, wantErrMsg %v", err.Error(), expectedErrorMsg)
				}
				return // Do not proceed to DeepEqual check if an error is expected
			}
			// Since we are checking for errors, we only compare 'got' and 'want' if no error is expected.
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewProxy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProxy_proxyRequest_MultiUpstream(t *testing.T) {
	// Common setup for multi-upstream tests
	createTestRequest := func(method, path string, body string) *http.Request {
		req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
		// Add common headers if necessary, e.g., for provider validation if secret is used
		req.Header.Add(providers.ContentTypeHeader, providers.DefaultContentTypeHeaderValue)
		return req
	}

	t.Run("BasicFanOut_BothSucceed_ReturnsFirstResponse", func(t *testing.T) {
		hitCounter1 := 0
		server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hitCounter1++
			w.Header().Set("X-Server-ID", "server1")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("response from server1"))
		}))
		defer server1.Close()

		hitCounter2 := 0
		server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hitCounter2++
			w.Header().Set("X-Server-ID", "server2")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("response from server2"))
		}))
		defer server2.Close()

		p, err := NewProxy(
			[]string{server1.URL, server2.URL},
			[]string{}, // Allow all paths
			providers.GithubProviderKind, // Using github for simplicity, no complex validation
			"", // No secret
			[]string{}, // No ignored users
		)
		if err != nil {
			t.Fatalf("Failed to create proxy: %v", err)
		}

		req := createTestRequest(http.MethodPost, "/testpath", "request body")
		rr := httptest.NewRecorder()
		// Need a router to match the /*path pattern
		router := httprouter.New()
		router.POST("/*path", p.proxyRequest)
		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		expectedBody := "response from server1"
		if rr.Body.String() != expectedBody {
			t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expectedBody)
		}
		expectedHeader := "server1"
		if rr.Header().Get("X-Server-ID") != expectedHeader {
			t.Errorf("handler returned unexpected X-Server-ID header: got %v want %v", rr.Header().Get("X-Server-ID"), expectedHeader)
		}

		if hitCounter1 != 1 {
			t.Errorf("server1 expected 1 hit, got %d", hitCounter1)
		}
		if hitCounter2 != 1 {
			t.Errorf("server2 expected 1 hit, got %d", hitCounter2)
		}
	})

	t.Run("FirstUpstreamFails_SecondSucceeds", func(t *testing.T) {
		hitCounter1 := 0
		server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hitCounter1++
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server1 error"))
		}))
		defer server1.Close()

		hitCounter2 := 0
		server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hitCounter2++
			w.Header().Set("X-Server-ID", "server2")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("response from server2"))
		}))
		defer server2.Close()

		p, err := NewProxy(
			[]string{server1.URL, server2.URL},
			[]string{}, providers.GithubProviderKind, "", []string{},
		)
		if err != nil {
			t.Fatalf("Failed to create proxy: %v", err)
		}

		req := createTestRequest(http.MethodPost, "/testpath", "request body")
		rr := httptest.NewRecorder()
		router := httprouter.New()
		router.POST("/*path", p.proxyRequest)
		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}
		expectedBody := "response from server2"
		if rr.Body.String() != expectedBody {
			t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expectedBody)
		}
		expectedHeader := "server2"
		if rr.Header().Get("X-Server-ID") != expectedHeader {
			t.Errorf("handler returned unexpected X-Server-ID header: got %v want %v", rr.Header().Get("X-Server-ID"), expectedHeader)
		}

		if hitCounter1 != 1 {
			t.Errorf("server1 expected 1 hit, got %d", hitCounter1)
		}
		if hitCounter2 != 1 {
			t.Errorf("server2 expected 1 hit, got %d", hitCounter2)
		}
	})

	t.Run("AllUpstreamsFail", func(t *testing.T) {
		hitCounter1 := 0
		server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hitCounter1++
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server1.Close()

		hitCounter2 := 0
		server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hitCounter2++
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server2.Close()

		p, err := NewProxy(
			[]string{server1.URL, server2.URL},
			[]string{}, providers.GithubProviderKind, "", []string{},
		)
		if err != nil {
			t.Fatalf("Failed to create proxy: %v", err)
		}

		req := createTestRequest(http.MethodPost, "/testpath", "request body")
		rr := httptest.NewRecorder()
		router := httprouter.New()
		router.POST("/*path", p.proxyRequest)
		router.ServeHTTP(rr, req)

		// As per current implementation, it should be InternalServerError
		if status := rr.Code; status != http.StatusInternalServerError {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusInternalServerError)
		}
		// Body should be "All upstream requests failed"
		expectedBody := "All upstream requests failed\n" // http.Error adds a newline
		if rr.Body.String() != expectedBody {
			t.Errorf("handler returned unexpected body: got '%v' want '%v'", rr.Body.String(), expectedBody)
		}

		if hitCounter1 != 1 {
			t.Errorf("server1 expected 1 hit, got %d", hitCounter1)
		}
		if hitCounter2 != 1 {
			t.Errorf("server2 expected 1 hit, got %d", hitCounter2)
		}
	})

	t.Run("RequestPathAndQueryPreservation", func(t *testing.T) {
		expectedPath := "/specific/path"
		expectedQuery := "param1=val1&param2=val2"
		fullRequestPath := expectedPath + "?" + expectedQuery

		pathAndQueryChecker := func(t *testing.T, serverName string, counter *int) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				(*counter)++
				if r.URL.Path != expectedPath {
					t.Errorf("%s received wrong path: got %s want %s", serverName, r.URL.Path, expectedPath)
				}
				if r.URL.RawQuery != expectedQuery {
					t.Errorf("%s received wrong query: got %s want %s", serverName, r.URL.RawQuery, expectedQuery)
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(serverName + " success"))
			}
		}

		hitCounter1 := 0
		server1 := httptest.NewServer(pathAndQueryChecker(t, "server1", &hitCounter1))
		defer server1.Close()

		hitCounter2 := 0
		server2 := httptest.NewServer(pathAndQueryChecker(t, "server2", &hitCounter2))
		defer server2.Close()

		p, err := NewProxy(
			[]string{server1.URL, server2.URL},
			[]string{}, providers.GithubProviderKind, "", []string{},
		)
		if err != nil {
			t.Fatalf("Failed to create proxy: %v", err)
		}

		req := createTestRequest(http.MethodPost, fullRequestPath, "request body for path/query test")
		rr := httptest.NewRecorder()
		router := httprouter.New()
		router.POST("/*path", p.proxyRequest) // Use /*path to capture the full path
		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}
		
		// Response should be from server1
		expectedBody := "server1 success"
		if rr.Body.String() != expectedBody {
			t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expectedBody)
		}

		if hitCounter1 != 1 {
			t.Errorf("server1 expected 1 hit, got %d", hitCounter1)
		}
		if hitCounter2 != 1 {
			t.Errorf("server2 expected 1 hit, got %d", hitCounter2)
		}
	})
	
	// Specific Response Content and Headers is implicitly tested by BasicFanOut and FirstUpstreamFails_SecondSucceeds
	// as they check for specific body and headers from the successful server.

	// No Upstream URLs test for NewProxy is already covered in TestNewProxy.
}

func TestProxy_isIgnoredUser(t *testing.T) {
	type fields struct {
		provider     string
		upstreamURLs []string
		allowedPaths []string
		secret       string
		ignoredUsers []string
	}
	type args struct {
		committer string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "TestIsIgnoredUserWithEmptyList",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2"},
				secret:       "secret",
				ignoredUsers: []string{},
			},
			args: args{
				committer: "user",
			},
			want: false,
		},
		{
			name: "TestIsIgnoredUserWithValidList",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2"},
				secret:       "secret",
				ignoredUsers: []string{"user1", "user2"},
			},
			args: args{
				committer: "user2",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Proxy{
				provider:     tt.fields.provider,
				upstreamURLs: tt.fields.upstreamURLs,
				allowedPaths: tt.fields.allowedPaths,
				secret:       tt.fields.secret,
				ignoredUsers: tt.fields.ignoredUsers,
			}
			if got := p.isIgnoredUser(tt.args.committer); got != tt.want {
				t.Errorf("Proxy.isIgnoredUser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProxy_isAllowedUser(t *testing.T) {
	type fields struct {
		provider     string
		upstreamURLs []string
		allowedPaths []string
		secret       string
		allowedUsers []string
	}
	type args struct {
		committer string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "TestIsAllowedUserWithEmptyList",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2"},
				secret:       "secret",
				allowedUsers: []string{},
			},
			args: args{
				committer: "user",
			},
			want: true,
		},
		{
			name: "TestIsAllowedUserWithValidList",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2"},
				secret:       "secret",
				allowedUsers: []string{"user1", "user2"},
			},
			args: args{
				committer: "user2",
			},
			want: true,
		},
		{
			name: "TestIsNotAllowedUserWithValidList",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2"},
				secret:       "secret",
				allowedUsers: []string{"user1", "user2"},
			},
			args: args{
				committer: "user3",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Proxy{
				provider:     tt.fields.provider,
				upstreamURLs: tt.fields.upstreamURLs,
				allowedPaths: tt.fields.allowedPaths,
				secret:       tt.fields.secret,
				allowedUsers: tt.fields.allowedUsers,
			}
			if got := p.isAllowedUser(tt.args.committer); got != tt.want {
				t.Errorf("Proxy.isAllowedUser() = %v, want %v", got, tt.want)
			}
		})
	}
}
