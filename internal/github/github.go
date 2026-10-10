package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Andrew-Sem/gc/specs/contract"
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
		http:   &http.Client{Timeout: 5 * time.Second},
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
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	}
	return false, fmt.Errorf("GitHub answered %d", resp.StatusCode)
}
