// Package tmdb — адаптер к The Movie Database, основному каталогу фильмов.
package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/arrowwhi/kinolist/internal/config"
	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

// cache вынесен в интерфейс на стороне потребителя: адаптеру всё равно,
// где лежат ответы, лишь бы их можно было положить и достать.
type cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, payload []byte, ttl time.Duration) error
}

type Client struct {
	http     *http.Client
	cache    cache
	baseURL  string
	apiKey   string
	language string
	cacheTTL time.Duration
}

func New(cfg config.TMDB, c cache) *Client {
	return &Client{
		http:     &http.Client{Timeout: cfg.Timeout},
		cache:    c,
		baseURL:  strings.TrimSuffix(cfg.BaseURL, "/"),
		apiKey:   cfg.APIKey,
		language: cfg.Language,
		cacheTTL: cfg.CacheTTL,
	}
}

func (c *Client) Provider() movie.Provider { return movie.ProviderTMDB }

func (c *Client) Search(ctx context.Context, title string, year int16) ([]movie.Candidate, error) {
	if strings.TrimSpace(title) == "" {
		return nil, movie.ErrEmptyQuery
	}

	params := url.Values{
		"query":         {title},
		"language":      {c.language},
		"include_adult": {"false"},
	}
	if year > 0 {
		params.Set("primary_release_year", strconv.Itoa(int(year)))
	}

	var resp searchResponse
	if err := c.get(ctx, "/search/movie", params, &resp); err != nil {
		return nil, err
	}

	// Год мог быть распознан неверно — если с ним ничего не нашлось,
	// повторяем без него, иначе теряем фильм из-за одной цифры.
	if len(resp.Results) == 0 && year > 0 {
		params.Del("primary_release_year")
		if err := c.get(ctx, "/search/movie", params, &resp); err != nil {
			return nil, err
		}
	}

	candidates := make([]movie.Candidate, 0, len(resp.Results))
	for _, r := range resp.Results {
		candidates = append(candidates, r.toCandidate())
	}

	return candidates, nil
}

func (c *Client) Details(ctx context.Context, externalID int32) (movie.Candidate, error) {
	params := url.Values{"language": {c.language}}

	var resp detailsResponse
	if err := c.get(ctx, "/movie/"+strconv.Itoa(int(externalID)), params, &resp); err != nil {
		return movie.Candidate{}, err
	}
	if resp.ID == 0 {
		return movie.Candidate{}, movie.ErrNotFound
	}

	return resp.toCandidate(), nil
}

func (c *Client) get(ctx context.Context, path string, params url.Values, out any) error {
	key := cacheKey(path, params)

	if c.cache != nil {
		if payload, ok, err := c.cache.Get(ctx, key); err == nil && ok {
			if err := json.Unmarshal(payload, out); err == nil {
				return nil
			}
			// Битую запись просто игнорируем и идём в сеть.
		}
	}

	endpoint := c.baseURL + path + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build tmdb request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	// TMDB принимает и v4-токен в Authorization, и v3-ключ в query.
	// Bearer работает для обоих форматов ключа v4; для v3 добавляем параметр.
	if strings.HasPrefix(c.apiKey, "ey") {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	} else {
		q := req.URL.Query()
		q.Set("api_key", c.apiKey)
		req.URL.RawQuery = q.Encode()
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: tmdb request failed: %w", movie.ErrProviderUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return movie.ErrNotFound
	case resp.StatusCode >= http.StatusInternalServerError, resp.StatusCode == http.StatusTooManyRequests:
		return fmt.Errorf("%w: tmdb returned %d", movie.ErrProviderUnavailable, resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("tmdb returned unexpected status %d", resp.StatusCode)
	}

	payload, err := readLimited(resp.Body)
	if err != nil {
		return fmt.Errorf("read tmdb response: %w", err)
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decode tmdb response: %w", err)
	}

	if c.cache != nil {
		// Ошибка записи в кэш не должна ронять успешный запрос.
		_ = c.cache.Set(ctx, key, payload, c.cacheTTL)
	}

	return nil
}

func cacheKey(path string, params url.Values) string {
	clean := url.Values{}
	for k, v := range params {
		clean[k] = v
	}
	return "tmdb:" + path + "?" + clean.Encode()
}
