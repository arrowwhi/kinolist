// Package llm вытаскивает название фильма из подписи к рилсу.
//
// Реализация читает только подпись — по ней распознаётся заметно меньше
// половины роликов, где название вшито в кадр. Кадры уже приходят в
// ReelContent, поэтому переход на Vision — это изменение одного запроса
// здесь, без правок домена, usecase и транспорта.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/arrowwhi/kinolist/internal/config"
	"github.com/arrowwhi/kinolist/internal/domain/enrichment"
)

const (
	maxResponseBytes = 1 << 20
	maxCaptionRunes  = 4000
	maxTokens        = 200
)

const systemPrompt = `Ты извлекаешь название фильма из подписи к посту в Instagram.
Подписи бывают на русском и английском, содержат эмодзи, хэштеги и рекламу.
Верни ТОЛЬКО JSON без markdown и пояснений:
{"title": "<название фильма>", "year": <год или 0>, "confidence": <0.0-1.0>}
Если в подписи нет конкретного фильма — верни {"title": "", "year": 0, "confidence": 0}.
Название верни так, как его пишут в кинобазах, без хэштегов и эмодзи.`

type Extractor struct {
	http    *http.Client
	baseURL string
	apiKey  string
	model   string
	enabled bool
}

func New(cfg config.LLM) *Extractor {
	return &Extractor{
		http:    &http.Client{Timeout: cfg.Timeout},
		baseURL: strings.TrimSuffix(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		enabled: cfg.Enabled(),
	}
}

func (e *Extractor) Enabled() bool { return e.enabled }

func (e *Extractor) Extract(ctx context.Context, content enrichment.ReelContent) (enrichment.ExtractedTitle, error) {
	if !e.enabled {
		return enrichment.ExtractedTitle{}, enrichment.ErrExtractorDisabled
	}

	caption := truncateRunes(strings.TrimSpace(content.Caption), maxCaptionRunes)
	if caption == "" {
		return enrichment.ExtractedTitle{}, nil
	}

	body, err := json.Marshal(request{
		Model:     e.model,
		MaxTokens: maxTokens,
		System:    systemPrompt,
		Messages:  []message{{Role: "user", Content: caption}},
	})
	if err != nil {
		return enrichment.ExtractedTitle{}, fmt.Errorf("encode llm request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return enrichment.ExtractedTitle{}, fmt.Errorf("build llm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", e.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := e.http.Do(req)
	if err != nil {
		return enrichment.ExtractedTitle{}, fmt.Errorf("llm request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return enrichment.ExtractedTitle{}, fmt.Errorf("read llm response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return enrichment.ExtractedTitle{}, fmt.Errorf("llm returned %d", resp.StatusCode)
	}

	var parsed response
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return enrichment.ExtractedTitle{}, fmt.Errorf("decode llm response: %w", err)
	}

	return parseExtraction(parsed.text())
}

type request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system"`
	Messages  []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type response struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func (r response) text() string {
	var b strings.Builder
	for _, c := range r.Content {
		if c.Type == "text" {
			b.WriteString(c.Text)
		}
	}
	return b.String()
}

type extraction struct {
	Title      string  `json:"title"`
	Year       int16   `json:"year"`
	Confidence float32 `json:"confidence"`
}

// parseExtraction терпит обёртку в ```json — модели добавляют её,
// несмотря на прямой запрет в системном промпте.
func parseExtraction(raw string) (enrichment.ExtractedTitle, error) {
	s := strings.TrimSpace(raw)
	if start := strings.Index(s, "{"); start >= 0 {
		if end := strings.LastIndex(s, "}"); end > start {
			s = s[start : end+1]
		}
	}
	if s == "" {
		return enrichment.ExtractedTitle{}, nil
	}

	var e extraction
	if err := json.Unmarshal([]byte(s), &e); err != nil {
		return enrichment.ExtractedTitle{}, fmt.Errorf("decode extraction %q: %w", truncateRunes(raw, 200), err)
	}

	title := strings.TrimSpace(e.Title)
	if title == "" {
		return enrichment.ExtractedTitle{}, nil
	}

	// Модель иногда возвращает год за пределами разумного — такому
	// значению доверять нельзя, а поиск без года всё равно отработает.
	if e.Year < 1888 || e.Year > 2100 {
		e.Year = 0
	}

	return enrichment.ExtractedTitle{Title: title, Year: e.Year, Confidence: e.Confidence}, nil
}

func truncateRunes(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit])
}
