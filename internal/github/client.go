package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func New(baseURL, token string, httpClient *http.Client) *Client {
	return &Client{baseURL: strings.TrimSuffix(baseURL, "/"), token: token, http: httpClient}
}

func (c *Client) RepositoryAccessible(ctx context.Context, repo string) (bool, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		return false, fmt.Errorf("имя репозитория не вида owner/name")
	}
	endpoint := c.baseURL + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.http.Do(req)
	if err != nil {
		return false, fmt.Errorf("GitHub недоступен: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("GitHub ответил %d", resp.StatusCode)
	}
}
