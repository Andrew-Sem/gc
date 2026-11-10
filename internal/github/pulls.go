package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Andrew-Sem/gc/specs/contract"
)

const mergeLimit = 60 * time.Second

type Pull struct {
	Number *int    `json:"number"`
	State  *string `json:"state"`
	Draft  *bool   `json:"draft"`
	Head   struct {
		SHA *string `json:"sha"`
	} `json:"head"`
	Base struct {
		Ref *string `json:"ref"`
	} `json:"base"`
	Mergeable      json.RawMessage `json:"mergeable"`
	MergeableState string          `json:"mergeable_state"`
	CreatedAt      *time.Time      `json:"created_at"`
	UpdatedAt      *time.Time      `json:"updated_at"`
}

func (p Pull) Snapshot(repository string) (contract.PrStateChanged, bool) {
	if p.Number == nil || p.State == nil || p.Draft == nil || p.Head.SHA == nil ||
		p.Base.Ref == nil || p.Mergeable == nil || p.CreatedAt == nil || p.UpdatedAt == nil {
		return contract.PrStateChanged{}, false
	}
	mergeable := contract.MergeableUnknown
	switch string(p.Mergeable) {
	case "true":
		mergeable = contract.MergeableYes
	case "false":
		mergeable = contract.MergeableNo
	}
	return contract.PrStateChanged{
		Repository: repository,
		Number:     *p.Number,
		HeadSHA:    *p.Head.SHA,
		Base:       *p.Base.Ref,
		Draft:      *p.Draft,
		Open:       *p.State == "open",
		Mergeable:  mergeable,
		CreatedAt:  p.CreatedAt.UTC(),
		UpdatedAt:  p.UpdatedAt.UTC(),
	}, true
}

func (c *Client) Pull(ctx context.Context, fullName, token string, number int) (Pull, error) {
	var pull Pull
	err := c.get(ctx, fmt.Sprintf("%s/pulls/%d", c.repositoryURL(fullName), number), token, &pull)
	return pull, err
}

func (c *Client) Merge(ctx context.Context, fullName, token string, number int, sha string) error {
	body, err := json.Marshal(map[string]string{"merge_method": "squash", "sha": sha})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, mergeLimit)
	defer cancel()
	resp, err := c.send(ctx, http.MethodPut, fmt.Sprintf("%s/pulls/%d/merge", c.repositoryURL(fullName), number), token, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return MergeRefused{Status: resp.StatusCode}
	}
	return nil
}

type MergeRefused struct {
	Status int
}

func (e MergeRefused) Error() string {
	return fmt.Sprintf("GitHub answered %d", e.Status)
}

func escapeBranch(branch string) string {
	parts := strings.Split(branch, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
