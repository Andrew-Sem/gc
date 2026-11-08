package github

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Andrew-Sem/gc/specs/contract"
)

const (
	lookupLimit   = 5 * time.Second
	createPRLimit = 60 * time.Second
)

var (
	ErrTokenRejected = errors.New("GitHub rejected the app's credentials")
	ErrRateLimited   = errors.New("GitHub rate limit reached")
	ErrFailed        = errors.New("GitHub failed")
	ErrMoved         = errors.New("GitHub moved the repository")
)

type Client struct {
	apiURL     string
	appID      string
	privateKey *rsa.PrivateKey
	http       *http.Client
}

func New(config contract.GitHub) *Client {
	return &Client{
		apiURL:     strings.TrimSuffix(config.APIURL, "/"),
		appID:      config.AppID,
		privateKey: config.PrivateKey,
		http: &http.Client{
			Transport:     config.Transport,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (c *Client) CanAccess(ctx context.Context, fullName string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, lookupLimit)
	defer cancel()
	_, err := c.installation(ctx, fullName)
	if errors.Is(err, errNotInstalled) {
		return false, nil
	}
	return err == nil, err
}

var errNotInstalled = errors.New("the app is not installed on the repository")

func (c *Client) installation(ctx context.Context, fullName string) (int64, error) {
	resp, err := c.send(ctx, http.MethodGet, c.repositoryURL(fullName)+"/installation", c.appToken(), nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusUnauthorized:
		return 0, ErrTokenRejected
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		return 0, ErrMoved
	case rateLimited(resp):
		return 0, ErrRateLimited
	case resp.StatusCode == http.StatusNotFound, resp.StatusCode == http.StatusForbidden:
		return 0, errNotInstalled
	default:
		return 0, fmt.Errorf("%w: answered %d", ErrFailed, resp.StatusCode)
	}
	var installation struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&installation); err != nil || installation.ID == 0 {
		return 0, fmt.Errorf("%w: no installation in the answer", ErrFailed)
	}
	return installation.ID, nil
}

func (c *Client) Token(ctx context.Context, fullName string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, lookupLimit)
	defer cancel()
	return c.token(ctx, fullName)
}

func (c *Client) token(ctx context.Context, fullName string) (string, error) {
	id, err := c.installation(ctx, fullName)
	if err != nil {
		return "", err
	}
	_, name, _ := strings.Cut(fullName, "/")
	body, err := json.Marshal(map[string]any{
		"repositories": []string{name},
		"permissions":  map[string]string{"contents": "write", "pull_requests": "write"},
	})
	if err != nil {
		return "", err
	}
	resp, err := c.send(ctx, http.MethodPost, fmt.Sprintf("%s/app/installations/%d/access_tokens", c.apiURL, id), c.appToken(), body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("%w: answered %d to a token request", ErrFailed, resp.StatusCode)
	}
	var token struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil || token.Token == "" {
		return "", fmt.Errorf("%w: no token in the answer", ErrFailed)
	}
	return token.Token, nil
}

func (c *Client) BotIdentity(ctx context.Context, token string) (name, email string, err error) {
	ctx, cancel := context.WithTimeout(ctx, lookupLimit)
	defer cancel()
	var app struct {
		Slug string `json:"slug"`
	}
	if err := c.get(ctx, c.apiURL+"/app", c.appToken(), &app); err != nil || app.Slug == "" {
		return "", "", fmt.Errorf("%w: cannot find the app's name", ErrFailed)
	}
	name = app.Slug + "[bot]"
	var user struct {
		ID int64 `json:"id"`
	}
	if err := c.get(ctx, c.apiURL+"/users/"+url.PathEscape(name), token, &user); err != nil || user.ID == 0 {
		return "", "", fmt.Errorf("%w: cannot find the app's bot user", ErrFailed)
	}
	return name, fmt.Sprintf("%d+%s@users.noreply.github.com", user.ID, name), nil
}

type OpenPR struct {
	Title string
	Files []string
}

func (c *Client) OpenPRs(ctx context.Context, fullName, token, branchPrefix string) ([]OpenPR, error) {
	ctx, cancel := context.WithTimeout(ctx, lookupLimit)
	defer cancel()
	var pulls []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Head   struct {
			Ref string `json:"ref"`
		} `json:"head"`
	}
	if err := c.get(ctx, c.repositoryURL(fullName)+"/pulls?state=open&per_page=100", token, &pulls); err != nil {
		return nil, err
	}
	var open []OpenPR
	for _, pull := range pulls {
		if !strings.HasPrefix(pull.Head.Ref, branchPrefix) {
			continue
		}
		var files []struct {
			Filename string `json:"filename"`
		}
		if err := c.get(ctx, fmt.Sprintf("%s/pulls/%d/files?per_page=100", c.repositoryURL(fullName), pull.Number), token, &files); err != nil {
			return nil, err
		}
		pr := OpenPR{Title: pull.Title}
		for _, file := range files {
			pr.Files = append(pr.Files, file.Filename)
		}
		open = append(open, pr)
	}
	return open, nil
}

func (c *Client) get(ctx context.Context, endpoint, token string, into any) error {
	resp, err := c.send(ctx, http.MethodGet, endpoint, token, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: answered %d", ErrFailed, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(into)
}

func (c *Client) CloneURL(ctx context.Context, fullName, token string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, lookupLimit)
	defer cancel()
	resp, err := c.send(ctx, http.MethodGet, c.repositoryURL(fullName), token, nil)
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
	body, err := json.Marshal(map[string]string{
		"head":  change.Branch,
		"base":  change.Base,
		"title": change.Title,
		"body":  change.Description,
	})
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, createPRLimit)
	defer cancel()
	token, err := c.token(ctx, fullName)
	if err != nil {
		return 0, err
	}
	resp, err := c.send(ctx, http.MethodPost, c.repositoryURL(fullName)+"/pulls", token, body)
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

func (c *Client) repositoryURL(fullName string) string {
	owner, name, _ := strings.Cut(fullName, "/")
	return c.apiURL + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
}

func (c *Client) appToken() string {
	now := time.Now()
	encode := base64.RawURLEncoding.EncodeToString
	header := encode([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]any{
		"iat": now.Add(-time.Minute).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": c.issuer(),
	})
	unsigned := header + "." + encode(claims)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, c.privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return ""
	}
	return unsigned + "." + encode(signature)
}

func (c *Client) issuer() any {
	if id, err := strconv.ParseInt(c.appID, 10, 64); err == nil {
		return id
	}
	return c.appID
}

func (c *Client) send(ctx context.Context, method, endpoint, token string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Close = true
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, unanswered(err)
	}
	return resp, nil
}

func unanswered(err error) error {
	var network *net.OpError
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return errors.New("GitHub did not answer in time")
	case errors.Is(err, context.Canceled):
		return context.Canceled
	case errors.As(err, &network):
		return fmt.Errorf("cannot reach GitHub: %w", network)
	}
	return errors.New("GitHub's answer is not valid HTTP")
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
