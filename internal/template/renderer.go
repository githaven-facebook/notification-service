package template

import (
	"context"
	"fmt"

	"github.com/nicedavid98/notification-service/internal/model"
)

// RenderResult holds the rendered title and body for a notification.
type RenderResult struct {
	Subject string
	Body    string
}

// Renderer wraps the template engine to render notifications.
type Renderer struct {
	engine *Engine
}

// NewRenderer creates a new notification renderer.
func NewRenderer(engine *Engine) *Renderer {
	return &Renderer{engine: engine}
}

// RenderNotification resolves and renders the template for a notification request.
// If no template is specified, the title and body from the request are used directly.
func (r *Renderer) RenderNotification(ctx context.Context, req *model.SendRequest) (*RenderResult, error) {
	if req.TemplateID == "" {
		return &RenderResult{
			Subject: req.Title,
			Body:    req.Body,
		}, nil
	}

	locale := req.Locale
	if locale == "" {
		locale = "en"
	}

	subject, body, err := r.engine.Render(ctx, req.TemplateID, req.Channel, locale, req.TemplateParams)
	if err != nil {
		return nil, fmt.Errorf("render notification template: %w", err)
	}

	// If template renders empty subject, fall back to request title
	if subject == "" {
		subject = req.Title
	}

	return &RenderResult{
		Subject: subject,
		Body:    body,
	}, nil
}
