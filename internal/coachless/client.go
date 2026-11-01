package coachless

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/go-resty/resty/v2"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

type Client struct {
	baseURL string
	http    *resty.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	rc := resty.New().
		SetTimeout(timeout).
		SetRetryCount(0)

	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    rc,
	}
}

func (c *Client) Refresh(ctx context.Context, refreshToken string) (domain.TokenPair, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return domain.TokenPair{}, errors.New("refresh token is required")
	}

	var (
		reqBody = map[string]string{"refreshToken": refreshToken}
		out     apiRefreshResponse
	)

	if err := c.doJSON(ctx, http.MethodPost, authRefreshPath, "", reqBody, &out); err != nil {
		return domain.TokenPair{}, err
	}

	accessToken := strings.TrimSpace(out.AccessToken)
	nextRefreshToken := strings.TrimSpace(out.RefreshToken)
	if accessToken == "" || nextRefreshToken == "" {
		return domain.TokenPair{}, errors.New("refresh response missing access or refresh token")
	}

	return domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: nextRefreshToken,
	}, nil
}

func (c *Client) GetPatches(ctx context.Context, accessToken string) ([]domain.PatchInfo, error) {
	var out []apiPatchInfo

	if err := c.doJSON(ctx, http.MethodGet, championWinprobPatchesPath, accessToken, nil, &out); err != nil {
		return nil, err
	}

	return patchInfosFromAPI(out), nil
}

func (c *Client) GetRecommendedBuilds(ctx context.Context, accessToken string, req domain.RecommendedBuildsRequest) (domain.RecommendedBuilds, error) {
	var out apiRecommendedBuildsResponse

	if err := c.doJSON(ctx, http.MethodPost, championWinprobRecommendedBuildsPath, accessToken, apiRecommendedBuildsRequestFromDomain(req), &out); err != nil {
		return domain.RecommendedBuilds{}, err
	}

	return recommendedBuildsFromAPI(out), nil
}

func (c *Client) doJSON(ctx context.Context, method, path, accessToken string, reqBody any, out any) error {
	operation := func() error {
		req := c.http.R().
			SetContext(ctx).
			SetHeader("Accept", "application/json, text/plain, */*").
			SetHeader("Content-Type", "application/json")

		if strings.TrimSpace(accessToken) != "" {
			req.SetAuthToken(accessToken)
		}

		if reqBody != nil {
			req.SetBody(reqBody)
		}

		resp, err := req.Execute(method, c.baseURL+path)
		if err != nil {
			return err
		}

		if resp.StatusCode() >= http.StatusInternalServerError {
			return fmt.Errorf("server error %d: %s", resp.StatusCode(), string(resp.Body()))
		}

		if resp.StatusCode() >= http.StatusBadRequest {
			return backoff.Permanent(fmt.Errorf("request failed %d: %s", resp.StatusCode(), string(resp.Body())))
		}

		if out != nil {
			if err := c.http.JSONUnmarshal(resp.Body(), out); err != nil {
				return backoff.Permanent(fmt.Errorf("decode response: %w", err))
			}
		}

		return nil
	}

	b := backoff.WithContext(backoff.NewExponentialBackOff(
		backoff.WithInitialInterval(200*time.Millisecond),
		backoff.WithMaxElapsedTime(2*time.Second),
	), ctx)

	if err := backoff.Retry(operation, b); err != nil {
		return fmt.Errorf("coachless %s %s: %w", method, path, err)
	}

	return nil
}
