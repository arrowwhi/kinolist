// Package instagram — адаптер к стороннему скраперу рилсов.
//
// Собственный клиент к instagram.com сюда сознательно не заводится: анонимный
// доступ упирается в login wall и бан IP датацентров, а залогиненная сессия
// нарушает ToS и рискует аккаунтом. Поэтому весь риск вынесен на внешний
// сервис, а адаптер оставлен тонким — сменить провайдера значит переписать
// только этот файл.
package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/arrowwhi/kinolist/internal/config"
	"github.com/arrowwhi/kinolist/internal/domain/enrichment"
)

const maxResponseBytes = 8 << 20

type Scraper struct {
	http    *http.Client
	baseURL string
	apiKey  string
	enabled bool
}

func New(cfg config.Instagram) *Scraper {
	return &Scraper{
		http:    &http.Client{Timeout: cfg.Timeout},
		baseURL: strings.TrimSuffix(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		enabled: cfg.Enabled(),
	}
}

// Enabled позволяет сервису подниматься без ключа скрапера: добавление
// по названию продолжает работать, ссылки отклоняются понятной ошибкой.
func (s *Scraper) Enabled() bool { return s.enabled }

func (s *Scraper) Fetch(ctx context.Context, reelURL string) (enrichment.ReelContent, error) {
	if !s.enabled {
		return enrichment.ReelContent{}, enrichment.ErrFetcherDisabled
	}

	endpoint := s.baseURL + "?" + url.Values{"url": {reelURL}}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return enrichment.ReelContent{}, fmt.Errorf("build scraper request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-KEY", s.apiKey)
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.http.Do(req)
	if err != nil {
		return enrichment.ReelContent{}, fmt.Errorf("%w: %w", enrichment.ErrReelUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusNotFound, resp.StatusCode == http.StatusGone:
		// Рилс удалён или приватный — повторять бессмысленно.
		return enrichment.ReelContent{}, enrichment.ErrInvalidReelURL
	case resp.StatusCode != http.StatusOK:
		return enrichment.ReelContent{}, fmt.Errorf("%w: scraper returned %d",
			enrichment.ErrReelUnavailable, resp.StatusCode)
	}

	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return enrichment.ReelContent{}, fmt.Errorf("read scraper response: %w", err)
	}

	content, err := parse(payload)
	if err != nil {
		return enrichment.ReelContent{}, err
	}
	if strings.TrimSpace(content.Caption) == "" && len(content.FrameURLs) == 0 {
		return enrichment.ReelContent{}, fmt.Errorf("%w: empty payload", enrichment.ErrReelUnavailable)
	}

	return content, nil
}

// response покрывает форматы, в которых распространённые обёртки отдают рилс.
// Скраперы регулярно переименовывают поля, поэтому читаем несколько вариантов
// и берём первое непустое.
type response struct {
	Caption     string `json:"caption"`
	CaptionText string `json:"caption_text"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Thumbnail   string `json:"thumbnail_url"`
	DisplayURL  string `json:"display_url"`
	ImageURL    string `json:"image_url"`
	Owner       string `json:"username"`
	Data        *struct {
		Caption     string `json:"caption"`
		CaptionText string `json:"caption_text"`
		Thumbnail   string `json:"thumbnail_url"`
		DisplayURL  string `json:"display_url"`
		Username    string `json:"username"`
	} `json:"data"`
}

func parse(payload []byte) (enrichment.ReelContent, error) {
	var r response
	if err := json.Unmarshal(payload, &r); err != nil {
		return enrichment.ReelContent{}, fmt.Errorf("decode scraper response: %w", err)
	}

	caption := firstNonEmpty(r.CaptionText, r.Caption, r.Description, r.Title)
	thumb := firstNonEmpty(r.Thumbnail, r.DisplayURL, r.ImageURL)
	author := r.Owner

	if r.Data != nil {
		caption = firstNonEmpty(caption, r.Data.CaptionText, r.Data.Caption)
		thumb = firstNonEmpty(thumb, r.Data.Thumbnail, r.Data.DisplayURL)
		author = firstNonEmpty(author, r.Data.Username)
	}

	content := enrichment.ReelContent{Caption: caption, Author: author}
	if thumb != "" {
		content.FrameURLs = []string{thumb}
	}

	return content, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
