package proxy

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/stakater/GitWebhookProxy/pkg/parser"
	"github.com/stakater/GitWebhookProxy/pkg/providers"
	"github.com/stakater/GitWebhookProxy/pkg/utils"
)

var (
	transport = &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient = &http.Client{
		Timeout:   time.Second * 30,
		Transport: transport,
	}
)

type Proxy struct {
	provider     string
	upstreamURLs []string
	allowedPaths []string
	secret       string
	ignoredUsers []string
	allowedUsers []string
}

func (p *Proxy) isPathAllowed(path string) bool {
	// All paths allowed
	if len(p.allowedPaths) == 0 {
		return true
	}

	// Check if given passed exists in allowedPaths
	for _, p := range p.allowedPaths {
		allowedPath := strings.TrimSpace(p)
		incomingPath := strings.TrimSpace(path)
		if strings.TrimSuffix(allowedPath, "/") ==
			strings.TrimSuffix(incomingPath, "/") || strings.HasPrefix(incomingPath, allowedPath) {
			return true
		}
	}
	return false
}

func (p *Proxy) isIgnoredUser(committer string) bool {
	if len(p.ignoredUsers) > 0 {
		if exists, _ := utils.InArray(p.ignoredUsers, committer); exists {
			return true
		}
	}

	if committer == "" && p.provider == providers.GithubName {
		return true
	}

	return false
}

func (p *Proxy) isAllowedUser(committer string) bool {
	if len(p.allowedUsers) > 0 {
		if exists, _ := utils.InArray(p.allowedUsers, committer); exists {
			return true
		}

		return false
	}

	return true
}

func (p *Proxy) redirect(hook *providers.Hook, redirectURL string) (*http.Response, error) {
	if hook == nil {
		return nil, errors.New("Cannot redirect with nil Hook")
	}

	// Parse url to check validity
	url, err := url.Parse(redirectURL)
	if err != nil {
		return nil, err
	}

	// Assign default scheme as http if not specified
	if url.Scheme == "" {
		url.Scheme = "http"
	}

	// Create Redirect request
	req, err := http.NewRequest(hook.RequestMethod, url.String(), bytes.NewBuffer(hook.Payload))

	if err != nil {
		return nil, err
	}

	// Set Headers from hook
	for key, value := range hook.Headers {
		req.Header.Add(key, value)
	}

	return httpClient.Do(req)

}

func (p *Proxy) proxyRequest(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	if !p.isPathAllowed(r.URL.Path) {
		log.Printf("Not allowed to proxy path: '%s'", r.URL.Path)
		http.Error(w, "Not allowed to proxy path: '"+r.URL.Path+"'", http.StatusForbidden)
		return
	}

	provider, err := providers.NewProvider(p.provider, p.secret)
	if err != nil {
		log.Printf("Error creating provider: %s", err)
		http.Error(w, "Error creating Provider", http.StatusInternalServerError)
		return
	}

	hook, err := parser.Parse(r, provider)
	if err != nil {
		log.Printf("Error Parsing Hook: %s", err)
		http.Error(w, "Error parsing Hook: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(p.ignoredUsers) > 0 || len(p.allowedUsers) > 0 {
		if event := provider.GetEventType(*hook); provider.IsCommitterCheckEvent(event) {
			committer := provider.GetCommitter(*hook, event)
			log.Printf("Incoming request from user: %s", committer)
			if p.isIgnoredUser(committer) || (!p.isAllowedUser(committer)) {
				log.Printf("Ignoring request for user: %s", committer)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(fmt.Sprintf("Ignoring request for user: %s", committer)))
				return
			}
		}
	}

	if len(strings.TrimSpace(p.secret)) > 0 && !provider.Validate(*hook) {
		log.Printf("Error Validating Hook: %v", err)
		http.Error(w, "Error validating Hook", http.StatusBadRequest)
		return
	}

	var responses []*http.Response
	var errorsList []error // Renamed to avoid conflict with the 'errors' package

	for _, upstream := range p.upstreamURLs {
		redirectURL := upstream + r.URL.Path
		if r.URL.RawQuery != "" {
			redirectURL += "?" + r.URL.RawQuery // Corrected query string concatenation
		}

		log.Printf("Proxying Request from '%s', to upstream '%s'\n", r.URL, redirectURL)
		resp, errRedirect := p.redirect(hook, redirectURL)
		responses = append(responses, resp)
		errorsList = append(errorsList, errRedirect) // Use renamed variable
	}

	var successfulResponse *http.Response
	var lastError error

	for i, resp := range responses {
		upstreamErr := errorsList[i]
		currentUpstreamURL := p.upstreamURLs[i] // For logging

		if upstreamErr != nil {
			log.Printf("Error redirecting to upstream '%s': %s\n", currentUpstreamURL, upstreamErr)
			lastError = upstreamErr // Keep track of errors
			if resp != nil && resp.Body != nil { // Ensure body is closed even if there was an error during request
				resp.Body.Close()
			}
			continue
		}

		if resp.StatusCode < 400 {
			log.Printf("Successfully redirected to upstream '%s' with status %s\n", currentUpstreamURL, resp.Status)
			successfulResponse = resp
			// Copy headers from this successful response to the client's response writer
			for key, values := range successfulResponse.Header {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
			// We will write body and status code after breaking the loop
			break // Found a successful response
		} else {
			log.Printf("Upstream '%s' returned error status: %s\n", currentUpstreamURL, resp.Status)
			lastError = fmt.Errorf("upstream %s returned status %s", currentUpstreamURL, resp.Status)
			if resp.Body != nil {
				resp.Body.Close()
			}
		}
	}

	if successfulResponse != nil {
		defer successfulResponse.Body.Close()
		responseBody, errReadBody := ioutil.ReadAll(successfulResponse.Body)
		if errReadBody != nil {
			log.Printf("Error reading response body from successful upstream: %s\n", errReadBody)
			http.Error(w, "Error reading response body", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(successfulResponse.StatusCode)
		w.Write(responseBody)
	} else {
		log.Printf("All upstream requests failed. Last error: %v\n", lastError)
		http.Error(w, "All upstream requests failed", http.StatusInternalServerError)
	}
}

// Health Check Endpoint
func (p *Proxy) health(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	w.WriteHeader(200)
	w.Write([]byte("I'm Healthy and I know it! ;) "))
}

// Run starts Proxy server
func (p *Proxy) Run(listenAddress string) error {
	if len(strings.TrimSpace(listenAddress)) == 0 {
		panic("Cannot create Proxy with empty listenAddress")
	}

	router := httprouter.New()
	router.GET("/health", p.health)
	router.POST("/*path", p.proxyRequest)

	log.Printf("Listening at: %s", listenAddress)
	return http.ListenAndServe(listenAddress, router)
}

func NewProxy(initialUpstreamURLs []string, allowedPaths []string,
	provider string, secret string, ignoredUsers []string) (*Proxy, error) {
	// Validate Params
	if initialUpstreamURLs == nil || len(initialUpstreamURLs) == 0 {
		return nil, errors.New("Cannot create Proxy with no upstreamURLs")
	}
	for _, u := range initialUpstreamURLs {
		if len(strings.TrimSpace(u)) == 0 {
			return nil, errors.New("Cannot create Proxy with an empty URL in upstreamURLs list")
		}
	}
	if len(strings.TrimSpace(provider)) == 0 {
		return nil, errors.New("Cannot create Proxy with empty provider")
	}
	if allowedPaths == nil {
		return nil, errors.New("Cannot create Proxy with nil allowedPaths")
	}

	return &Proxy{
		provider:     provider,
		upstreamURLs: initialUpstreamURLs,
		allowedPaths: allowedPaths,
		secret:       secret,
		ignoredUsers: ignoredUsers,
	}, nil
}
