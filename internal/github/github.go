package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Andrew-Sem/gc/specs/contract"
)

var (
	ErrTokenRejected = errors.New("GitHub rejected the token")
	ErrRateLimited   = errors.New("GitHub rate limit reached")
	ErrFailed        = errors.New("GitHub failed")
	ErrMoved         = errors.New("GitHub moved the repository")
)

type Client struct {
	apiURL string
	token  string
	http   *http.Client
}

func New(config contract.GitHub) *Client {
	return &Client{
		apiURL: strings.TrimSuffix(config.APIURL, "/"),
		token:  config.Token,
		http: &http.Client{
			Transport:     config.Transport,
			Timeout:       5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (c *Client) CanAccess(ctx context.Context, fullName string) (bool, error) {
	owner, name, _ := strings.Cut(fullName, "/")
	endpoint := c.apiURL + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.http.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusOK:
		return true, nil
	case resp.StatusCode == http.StatusUnauthorized:
		return false, ErrTokenRejected
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		return false, ErrMoved
	case rateLimited(resp):
		return false, ErrRateLimited
	case resp.StatusCode == http.StatusNotFound, resp.StatusCode == http.StatusForbidden:
		return false, nil
	}
	return false, fmt.Errorf("%w: answered %d", ErrFailed, resp.StatusCode)
}

func (c *Client) CloneURL(ctx context.Context, fullName string) (string, error) {
	owner, name, _ := strings.Cut(fullName, "/")
	endpoint := c.apiURL + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: answered %d", ErrFailed, resp.StatusCode)
	}
	var repository struct {
		CloneURL string `json:"clone_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repository); err != nil {
		return "", fmt.Errorf("%w: %v", ErrFailed, err)
	}
	if repository.CloneURL == "" {
		return "", fmt.Errorf("%w: no clone URL in the answer", ErrFailed)
	}
	return repository.CloneURL, nil
}

func (c *Client) CreatePR(ctx context.Context, fullName string, change contract.AgentChange) (int, error) {
	owner, name, _ := strings.Cut(fullName, "/")
	endpoint := c.apiURL + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name) + "/pulls"
	body, err := json.Marshal(map[string]string{
		"head":  change.Branch,
		"base":  change.Base,
		"title": change.Title,
		"body":  change.Description,
	})
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return 0, fmt.Errorf("%w: answered %d", ErrFailed, resp.StatusCode)
	}
	var created struct {
		Number int `json:"number"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrFailed, err)
	}
	if created.Number == 0 {
		return 0, fmt.Errorf("%w: no PR number in the answer", ErrFailed)
	}
	return created.Number, nil
}

func rateLimited(resp *http.Response) bool {
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return true
	case http.StatusForbidden:
		return resp.Header.Get("X-RateLimit-Remaining") == "0" || resp.Header.Get("Retry-After") != ""
	}
	return false
}
