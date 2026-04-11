package template

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"text/template"
	"time"

	"go.uber.org/zap"

	"github.com/nicedavid98/notification-service/internal/model"
	"github.com/nicedavid98/notification-service/internal/repository"
)

// cacheEntry holds a compiled template and its expiry.
type cacheEntry struct {
	tmpl      *template.Template
	expiresAt time.Time
	subject   string
}

// Engine manages template loading, compilation, caching, and rendering.
type Engine struct {
	repo   repository.TemplateRepository
	cache  map[string]*cacheEntry
	mu     sync.RWMutex
	ttl    time.Duration
	logger *zap.Logger
}

// NewEngine creates a new template engine.
func NewEngine(repo repository.TemplateRepository, logger *zap.Logger) *Engine {
	return &Engine{
		repo:   repo,
		cache:  make(map[string]*cacheEntry),
		ttl:    5 * time.Minute,
		logger: logger,
	}
}

// Render renders a template identified by name, channel, and locale with the given params.
// It falls back through locale hierarchy: e.g., "ko-KR" -> "ko" -> "en".
func (e *Engine) Render(ctx context.Context, name string, channel model.NotificationChannel, locale string, params map[string]string) (subject, body string, err error) {
	locales := buildLocaleFallbackChain(locale)

	var tmplRecord *model.NotificationTemplate
	for _, loc := range locales {
		tmplRecord, err = e.getTemplate(ctx, name, channel, loc)
		if err != nil {
			return "", "", fmt.Errorf("get template %s/%s/%s: %w", name, channel, loc, err)
		}
		if tmplRecord != nil {
			break
		}
	}

	if tmplRecord == nil {
		return "", "", fmt.Errorf("template not found: name=%s channel=%s locale=%s", name, channel, locale)
	}

	cacheKey := buildCacheKey(name, string(channel), tmplRecord.Locale, tmplRecord.Version)

	compiledSubject, compiledBody, err := e.getCompiledTemplate(cacheKey, tmplRecord)
	if err != nil {
		return "", "", err
	}

	// Build template data map
	data := make(map[string]interface{}, len(params))
	for k, v := range params {
		data[k] = v
	}

	subject, err = renderTemplate(compiledSubject, data)
	if err != nil {
		return "", "", fmt.Errorf("render template subject: %w", err)
	}

	body, err = renderTemplate(compiledBody, data)
	if err != nil {
		return "", "", fmt.Errorf("render template body: %w", err)
	}

	return subject, body, nil
}

// getTemplate fetches a template from cache or database.
func (e *Engine) getTemplate(ctx context.Context, name string, channel model.NotificationChannel, locale string) (*model.NotificationTemplate, error) {
	return e.repo.GetByNameAndLocale(ctx, name, channel, locale)
}

// getCompiledTemplate returns a compiled template from cache, compiling if necessary.
func (e *Engine) getCompiledTemplate(cacheKey string, tmpl *model.NotificationTemplate) (*template.Template, *template.Template, error) {
	e.mu.RLock()
	entry, ok := e.cache[cacheKey]
	e.mu.RUnlock()

	if ok && time.Now().Before(entry.expiresAt) {
		subjectTmpl, err := template.New("subject").Parse(entry.subject)
		if err != nil {
			return nil, nil, fmt.Errorf("compile cached subject template: %w", err)
		}
		return subjectTmpl, entry.tmpl, nil
	}

	// Compile body template
	bodyTmpl, err := template.New("body").Parse(tmpl.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("compile body template: %w", err)
	}

	// Compile subject template
	subjectTmpl, err := template.New("subject").Parse(tmpl.Subject)
	if err != nil {
		return nil, nil, fmt.Errorf("compile subject template: %w", err)
	}

	e.mu.Lock()
	e.cache[cacheKey] = &cacheEntry{
		tmpl:      bodyTmpl,
		subject:   tmpl.Subject,
		expiresAt: time.Now().Add(e.ttl),
	}
	e.mu.Unlock()

	return subjectTmpl, bodyTmpl, nil
}

// InvalidateCache removes all cached templates.
func (e *Engine) InvalidateCache() {
	e.mu.Lock()
	e.cache = make(map[string]*cacheEntry)
	e.mu.Unlock()
}

// InvalidateCacheEntry removes a specific cached template.
func (e *Engine) InvalidateCacheEntry(name string, channel model.NotificationChannel, locale string, version int) {
	cacheKey := buildCacheKey(name, string(channel), locale, version)
	e.mu.Lock()
	delete(e.cache, cacheKey)
	e.mu.Unlock()
}

// buildLocaleFallbackChain builds a locale fallback chain.
// Example: "ko-KR" -> ["ko-KR", "ko", "en"]
func buildLocaleFallbackChain(locale string) []string {
	if locale == "" {
		return []string{"en"}
	}

	chain := []string{locale}

	// If locale is like "ko-KR", add "ko"
	if idx := strings.Index(locale, "-"); idx != -1 {
		lang := locale[:idx]
		chain = append(chain, lang)
	}

	// Always fall back to English
	if locale != "en" {
		chain = append(chain, "en")
	}

	// Deduplicate
	seen := make(map[string]struct{})
	result := make([]string, 0, len(chain))
	for _, l := range chain {
		if _, ok := seen[l]; !ok {
			seen[l] = struct{}{}
			result = append(result, l)
		}
	}
	return result
}

// buildCacheKey constructs a unique cache key for a template.
func buildCacheKey(name, channel, locale string, version int) string {
	return fmt.Sprintf("%s:%s:%s:%d", name, channel, locale, version)
}

// renderTemplate executes a compiled template with the given data.
func renderTemplate(tmpl *template.Template, data map[string]interface{}) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}
