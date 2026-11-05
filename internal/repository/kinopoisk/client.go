// Package kinopoisk — запасной каталог на неофициальном API Кинопоиска.
// Используется, когда TMDB ничего не нашёл: русское и советское кино
// там представлено заметно лучше.
package kinopoisk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/arrowwhi/kinolist/internal/config"
	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

const maxResponseBytes = 4 << 20

type Client struct {
	http    *http.Client
	baseURL string
	apiKey  string
}

func New(cfg config.Kinopoisk) *Client {
	return &Client{
		http:    &http.Client{Timeout: cfg.Timeout},
		baseURL: strings.TrimSuffix(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
	}
}

func (c *Client) Provider() movie.Provider { return movie.ProviderKinopoisk }

func (c *Client) Search(ctx context.Context, title string, year int16) ([]movie.Candidate, error) {
	if strings.TrimSpace(title) == "" {
		return nil, movie.ErrEmptyQuery
	}

	params := url.Values{"keyword": {title}, "page": {"1"}}

	var resp searchResponse
	if err := c.get(ctx, "/api/v2.1/films/search-by-keyword", params, &resp); err != nil {
		return nil, err
	}

	candidates := make([]movie.Candidate, 0, len(resp.Films))
	for _, f := range resp.Films {
		if v := f.toCandidate(); v.MediaType.Valid() {
			candidates = append(candidates, v)
		}
	}

	// API не умеет фильтровать по году, поэтому сужаем выдачу сами —
	// но только если после фильтра что-то осталось.
	if year > 0 {
		filtered := make([]movie.Candidate, 0, len(candidates))
		for _, c := range candidates {
			if c.Year == year {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) > 0 {
			return filtered, nil
		}
	}

	return candidates, nil
}

func (c *Client) Details(ctx context.Context, externalID int32, mediaTypes ...movie.MediaType) (movie.Candidate, error) {
	var resp filmDetails
	if err := c.get(ctx, "/api/v2.2/films/"+strconv.Itoa(int(externalID)), nil, &resp); err != nil {
		return movie.Candidate{}, err
	}
	if resp.KinopoiskID == 0 {
		return movie.Candidate{}, movie.ErrNotFound
	}

	d := resp.toCandidate()
	if !d.MediaType.Valid() {
		return movie.Candidate{}, movie.ErrNotFound
	}
	if len(mediaTypes) > 0 && d.MediaType != mediaTypes[0] {
		return movie.Candidate{}, movie.ErrNotFound
	}
	if d.MediaType == movie.MediaTV {
		d.RuntimeMin = 0
		var seasons struct {
			Items []struct {
				Number int16 `json:"number"`
			} `json:"items"`
		}
		if err := c.get(ctx, "/api/v2.2/films/"+strconv.Itoa(int(externalID))+"/seasons", nil, &seasons); err == nil {
			count := int16(0)
			seen := map[int16]bool{}
			for _, s := range seasons.Items {
				if s.Number > 0 && !seen[s.Number] {
					count++
					seen[s.Number] = true
				}
			}
			if count > 0 {
				d.SeasonCount = &count
			}
		}
	}
	return d, nil
}

func (c *Client) get(ctx context.Context, path string, params url.Values, out any) error {
	endpoint := c.baseURL + path
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build kinopoisk request: %w", err)
	}
	req.Header.Set("X-API-KEY", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: kinopoisk request failed: %w", movie.ErrProviderUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return movie.ErrNotFound
	case resp.StatusCode >= http.StatusInternalServerError, resp.StatusCode == http.StatusTooManyRequests:
		return fmt.Errorf("%w: kinopoisk returned %d", movie.ErrProviderUnavailable, resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("kinopoisk returned unexpected status %d", resp.StatusCode)
	}

	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("read kinopoisk response: %w", err)
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decode kinopoisk response: %w", err)
	}

	return nil
}
