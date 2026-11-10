package github

import (
	"context"
	"fmt"
	"time"

	"github.com/Andrew-Sem/gc/specs/contract"
)

const checkRunsPage = 100

type CheckRun struct {
	ID   *int64  `json:"id"`
	Name *string `json:"name"`
	App  *struct {
		ID *int64 `json:"id"`
	} `json:"app"`
	HeadSHA     *string    `json:"head_sha"`
	Status      *string    `json:"status"`
	Conclusion  *string    `json:"conclusion"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

func (r CheckRun) Snapshot() (contract.CheckRun, bool) {
	if r.ID == nil || r.Name == nil || r.App == nil || r.App.ID == nil || r.HeadSHA == nil || r.Status == nil || r.StartedAt == nil {
		return contract.CheckRun{}, false
	}
	run := contract.CheckRun{
		ID: *r.ID, Name: *r.Name, AppID: *r.App.ID, HeadSHA: *r.HeadSHA, Status: *r.Status,
		StartedAt: r.StartedAt.UTC(),
	}
	if r.Conclusion != nil {
		run.Conclusion = *r.Conclusion
	}
	if r.CompletedAt != nil {
		run.CompletedAt = r.CompletedAt.UTC()
	}
	return run, true
}

func (c *Client) RequiredChecks(ctx context.Context, fullName, token, branch string) ([]contract.RequiredCheck, error) {
	var answer struct {
		Protection struct {
			RequiredStatusChecks struct {
				Contexts []string `json:"contexts"`
				Checks   []struct {
					Context string `json:"context"`
					AppID   *int64 `json:"app_id"`
				} `json:"checks"`
			} `json:"required_status_checks"`
		} `json:"protection"`
	}
	if err := c.get(ctx, c.repositoryURL(fullName)+"/branches/"+escapeBranch(branch), token, &answer); err != nil {
		return nil, err
	}
	required := []contract.RequiredCheck{}
	checks := answer.Protection.RequiredStatusChecks
	if len(checks.Checks) > 0 {
		for _, check := range checks.Checks {
			appID := int64(0)
			if check.AppID != nil && *check.AppID > 0 {
				appID = *check.AppID
			}
			required = append(required, contract.RequiredCheck{Name: check.Context, AppID: appID})
		}
		return required, nil
	}
	for _, name := range checks.Contexts {
		required = append(required, contract.RequiredCheck{Name: name})
	}
	return required, nil
}

func (c *Client) CheckRuns(ctx context.Context, fullName, token, head string) ([]contract.CheckRun, error) {
	var answer struct {
		TotalCount int        `json:"total_count"`
		CheckRuns  []CheckRun `json:"check_runs"`
	}
	endpoint := fmt.Sprintf("%s/commits/%s/check-runs?per_page=%d", c.repositoryURL(fullName), escapeBranch(head), checkRunsPage)
	if err := c.get(ctx, endpoint, token, &answer); err != nil {
		return nil, err
	}
	if answer.TotalCount > len(answer.CheckRuns) {
		return nil, fmt.Errorf("%w: %d check runs, more than one page", ErrFailed, answer.TotalCount)
	}
	runs := []contract.CheckRun{}
	for _, raw := range answer.CheckRuns {
		run, complete := raw.Snapshot()
		if !complete {
			return nil, fmt.Errorf("%w: a check run without its fields", ErrFailed)
		}
		runs = append(runs, run)
	}
	return runs, nil
}
