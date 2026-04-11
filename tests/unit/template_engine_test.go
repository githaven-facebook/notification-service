package unit

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/nicedavid98/notification-service/internal/model"
	tmpl "github.com/nicedavid98/notification-service/internal/template"
)

func TestTemplateEngine_RenderBasic(t *testing.T) {
	repo := newMockTemplateRepo()
	logger, _ := zap.NewDevelopment()
	engine := tmpl.NewEngine(repo, logger)

	// Seed template
	_ = repo.Create(context.Background(), &model.NotificationTemplate{
		Name:    "welcome",
		Channel: model.ChannelFCM,
		Subject: "Welcome {{index . \"UserName\"}}!",
		Body:    "Hi {{index . \"UserName\"}}, welcome to {{index . \"AppName\"}}.",
		Locale:  "en",
		Version: 1,
		Active:  true,
	})

	subject, body, err := engine.Render(context.Background(), "welcome", model.ChannelFCM, "en", map[string]string{
		"UserName": "Alice",
		"AppName":  "TestApp",
	})
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	if subject == "" {
		t.Error("expected non-empty subject")
	}
	if body == "" {
		t.Error("expected non-empty body")
	}
}

func TestTemplateEngine_LocaleFallback(t *testing.T) {
	repo := newMockTemplateRepo()
	logger, _ := zap.NewDevelopment()
	engine := tmpl.NewEngine(repo, logger)

	// Only seed English template
	_ = repo.Create(context.Background(), &model.NotificationTemplate{
		Name:    "welcome",
		Channel: model.ChannelFCM,
		Subject: "Welcome!",
		Body:    "Welcome to the app.",
		Locale:  "en",
		Version: 1,
		Active:  true,
	})

	// Request ko-KR, should fall back to en
	_, body, err := engine.Render(context.Background(), "welcome", model.ChannelFCM, "ko-KR", map[string]string{})
	if err != nil {
		t.Fatalf("expected locale fallback to succeed, got: %v", err)
	}
	if body == "" {
		t.Error("expected non-empty body after locale fallback")
	}
}

func TestTemplateEngine_LocaleKoFallback(t *testing.T) {
	repo := newMockTemplateRepo()
	logger, _ := zap.NewDevelopment()
	engine := tmpl.NewEngine(repo, logger)

	// Seed ko template (not ko-KR)
	_ = repo.Create(context.Background(), &model.NotificationTemplate{
		Name:    "welcome",
		Channel: model.ChannelFCM,
		Subject: "환영합니다!",
		Body:    "앱에 오신 것을 환영합니다.",
		Locale:  "ko",
		Version: 1,
		Active:  true,
	})

	// Request ko-KR, should fall back to ko
	_, body, err := engine.Render(context.Background(), "welcome", model.ChannelFCM, "ko-KR", map[string]string{})
	if err != nil {
		t.Fatalf("expected ko-KR to fall back to ko, got: %v", err)
	}
	if body != "앱에 오신 것을 환영합니다." {
		t.Errorf("expected Korean body, got: %s", body)
	}
}

func TestTemplateEngine_NotFound(t *testing.T) {
	repo := newMockTemplateRepo()
	logger, _ := zap.NewDevelopment()
	engine := tmpl.NewEngine(repo, logger)

	_, _, err := engine.Render(context.Background(), "nonexistent", model.ChannelFCM, "en", map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing template")
	}
}

func TestTemplateEngine_CacheInvalidation(t *testing.T) {
	repo := newMockTemplateRepo()
	logger, _ := zap.NewDevelopment()
	engine := tmpl.NewEngine(repo, logger)

	_ = repo.Create(context.Background(), &model.NotificationTemplate{
		Name:    "test",
		Channel: model.ChannelSES,
		Subject: "Test",
		Body:    "Original body",
		Locale:  "en",
		Version: 1,
		Active:  true,
	})

	_, body1, err := engine.Render(context.Background(), "test", model.ChannelSES, "en", map[string]string{})
	if err != nil {
		t.Fatalf("first render failed: %v", err)
	}
	if body1 != "Original body" {
		t.Errorf("unexpected body: %s", body1)
	}

	// Invalidate cache
	engine.InvalidateCache()

	// Update template in repo
	repo.templates["test:ses:en"].Body = "Updated body"

	_, body2, err := engine.Render(context.Background(), "test", model.ChannelSES, "en", map[string]string{})
	if err != nil {
		t.Fatalf("second render failed: %v", err)
	}
	if body2 != "Updated body" {
		t.Errorf("expected updated body after cache invalidation, got: %s", body2)
	}
}

func TestValidateSyntax_Valid(t *testing.T) {
	err := tmpl.ValidateSyntax("body", "Hello {{index . \"Name\"}}!")
	if err != nil {
		t.Errorf("expected valid template syntax, got: %v", err)
	}
}

func TestValidateSyntax_Invalid(t *testing.T) {
	err := tmpl.ValidateSyntax("body", "Hello {{.Name")
	if err == nil {
		t.Error("expected error for invalid template syntax")
	}
}

func TestValidateSyntax_Empty(t *testing.T) {
	err := tmpl.ValidateSyntax("body", "")
	if err == nil {
		t.Error("expected error for empty template")
	}
}

func TestValidateRequiredVariables(t *testing.T) {
	params := map[string]string{
		"Name": "Alice",
	}

	// Should pass: all required vars present
	err := tmpl.ValidateRequiredVariables("Hello", params, []string{"Name"})
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Should fail: missing required var
	err = tmpl.ValidateRequiredVariables("Hello", params, []string{"Name", "Email"})
	if err == nil {
		t.Error("expected error for missing required variable")
	}
}
