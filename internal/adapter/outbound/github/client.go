package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/user/github-release-notification-api/internal/domain"
)

type Client struct {
	httpClient *http.Client
	token      string
	baseURL    string
	maxRetries int
	logger     *slog.Logger
}

func NewClient(token, baseURL string, timeout time.Duration, maxRetries int, logger *slog.Logger) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		token:      token,
		baseURL:    baseURL,
		maxRetries: maxRetries,
		logger:     logger,
	}
}

func (c *Client) RepoExists(ctx context.Context, owner, repo string) (bool, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)

	resp, err := c.doRequestWithRetry(ctx, url)
	if err != nil {
		return false, err
	}
	defer c.closeRespBody(resp, url)

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("unexpected status code %d from GitHub API", resp.StatusCode)
	}
}

func (c *Client) GetLatestRelease(ctx context.Context, owner, repo string) (*domain.Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.baseURL, owner, repo)

	resp, err := c.doRequestWithRetry(ctx, url)
	if err != nil {
		return nil, err
	}
	defer c.closeRespBody(resp, url)

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d from GitHub API", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var ghRelease struct {
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		HTMLURL     string `json:"html_url"`
		PublishedAt string `json:"published_at"`
	}
	if err := json.Unmarshal(body, &ghRelease); err != nil {
		return nil, fmt.Errorf("parsing release JSON: %w", err)
	}

	release := &domain.Release{
		TagName: ghRelease.TagName,
		Name:    ghRelease.Name,
		HTMLURL: ghRelease.HTMLURL,
	}
	if ghRelease.PublishedAt != "" {
		t, err := time.Parse(time.RFC3339, ghRelease.PublishedAt)
		if err == nil {
			release.PublishedAt = t
		}
	}

	return release, nil
}

func (c *Client) doRequestWithRetry(ctx context.Context, url string) (*http.Response, error) {
	backoff := 1 * time.Second

	for attempt := 0; attempt < c.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if !isTransientNetworkError(err) {
				return nil, fmt.Errorf("executing request: %w", err)
			}

			c.logger.Warn("transient network error, retrying",
				"attempt", attempt+1,
				"url", url,
				"error", err,
			)
			if err := c.sleepWithContext(ctx, backoff); err != nil {
				return nil, err
			}
			backoff *= 2
			continue
		}

		if isServerError(resp.StatusCode) {
			c.closeRespBody(resp, url)
			c.logger.Warn("GitHub API server error, retrying",
				"attempt", attempt+1,
				"status", resp.StatusCode,
				"url", url,
			)
			if err := c.sleepWithContext(ctx, backoff); err != nil {
				return nil, err
			}
			backoff *= 2
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests ||
			(resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0") {
			c.closeRespBody(resp, url)

			waitDuration := c.calcRateLimitWait(resp, backoff)

			c.logger.Warn("GitHub API rate limited, retrying",
				"attempt", attempt+1,
				"wait", waitDuration.String(),
				"url", url,
			)

			if err := c.sleepWithContext(ctx, waitDuration); err != nil {
				return nil, err
			}
			backoff *= 2
			continue
		}

		return resp, nil
	}

	return nil, domain.ErrRateLimited
}

func (c *Client) closeRespBody(resp *http.Response, url string) {
	if resp == nil || resp.Body == nil {
		return
	}
	if err := resp.Body.Close(); err != nil {
		c.logger.Debug("close response body", "url", url, "error", err)
	}
}

func (c *Client) calcRateLimitWait(resp *http.Response, fallback time.Duration) time.Duration {
	const maxWait = 60 * time.Second

	waitDuration := fallback
	if resetStr := resp.Header.Get("X-RateLimit-Reset"); resetStr != "" {
		if resetUnix, err := strconv.ParseInt(resetStr, 10, 64); err == nil {
			resetTime := time.Unix(resetUnix, 0)
			w := time.Until(resetTime) + time.Second
			if w > 0 {
				waitDuration = w
			}
		}
	}
	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			waitDuration = time.Duration(seconds) * time.Second
		}
	}
	if waitDuration > maxWait {
		waitDuration = maxWait
	}
	return waitDuration
}

func (c *Client) sleepWithContext(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

func isServerError(statusCode int) bool {
	return statusCode >= 500 && statusCode <= 599
}

func isTransientNetworkError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr)
}
