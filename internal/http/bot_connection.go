package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"fahd-backend/internal/ai"
	"fahd-backend/internal/ai/providers"

	"github.com/gin-gonic/gin"
)

var googleModelID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

func normalizeBotConnectionPayload(payload map[string]any) error {
	if raw, ok := payload["provider"]; ok {
		provider, ok := raw.(string)
		if !ok {
			return fmt.Errorf("اختر Google AI Studio كمزوّد الاتصال")
		}
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider != "google" && provider != "gemini" {
			return fmt.Errorf("اختر Google AI Studio كمزوّد الاتصال")
		}
		payload["provider"] = "google"
	}
	if raw, ok := payload["model"]; ok {
		model, ok := raw.(string)
		if !ok {
			return fmt.Errorf("اسم الموديل غير صحيح")
		}
		model = ai.NormalizeGoogleModel(model)
		if !googleModelID.MatchString(model) {
			return fmt.Errorf("اسم الموديل غير صحيح؛ اختر موديلًا من القائمة")
		}
		payload["model"] = model
	}
	_, hasCamel := payload["apiKey"]
	_, hasSnake := payload["api_key"]
	if hasCamel && hasSnake {
		return fmt.Errorf("أرسل مفتاح API مرة واحدة فقط")
	}
	for _, key := range []string{"apiKey", "api_key"} {
		if raw, exists := payload[key]; exists {
			value, ok := raw.(string)
			if !ok {
				return fmt.Errorf("أدخل مفتاح Google AI Studio كنص")
			}
			value = strings.TrimSpace(value)
			if strings.IndexFunc(value, unicode.IsSpace) >= 0 {
				return fmt.Errorf("مفتاح API يحتوي على مسافات داخلية؛ انسخه كاملًا من Google AI Studio")
			}
			payload[key] = value
		}
	}
	return nil
}

// Connection tests accept a draft key without replacing the working saved key.
// Omitting apiKey uses the stored encrypted key and preserves it unchanged.
func (h *Handler) botConnectionConfig(c *gin.Context, draft map[string]any) (ai.BotConfig, bool) {
	if err := normalizeBotConnectionPayload(draft); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "code": "invalid_config", "error": err.Error()})
		return ai.BotConfig{}, false
	}
	cfg, err := ai.LoadBotConfig(h.db.WithContext(c.Request.Context()), h.cfg.JWTSecret)
	_, hasDraftKey := draft["apiKey"]
	_, hasSnakeKey := draft["api_key"]
	hasDraftKey = hasDraftKey || hasSnakeKey
	if err != nil {
		if !hasDraftKey {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "code": "missing_key", "error": "تعذر قراءة المفتاح المحفوظ. أدخل مفتاح Google AI Studio وأعد الاختبار."})
			return ai.BotConfig{}, false
		}
		cfg = ai.BotConfig{Provider: "google", Model: ai.DefaultGoogleModel}
	}
	if value, ok := draft["provider"].(string); ok {
		cfg.Provider = value
	}
	if value, ok := draft["model"].(string); ok {
		cfg.Model = value
	}
	if value, ok := draft["apiKey"].(string); ok {
		cfg.APIKey = value
	}
	if value, ok := draft["api_key"].(string); ok {
		cfg.APIKey = value
	}
	if cfg.Provider != "google" || cfg.APIKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "code": "missing_key", "error": "أدخل مفتاح Google AI Studio أولًا. ترك الحقل فارغًا يحافظ على المفتاح المحفوظ."})
		return ai.BotConfig{}, false
	}
	return cfg, true
}

func (h *Handler) AdminGoogleModels(c *gin.Context) {
	cfg, ok := h.botConnectionConfig(c, map[string]any{})
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	models, err := providers.NewGoogleProvider(providers.GoogleConfig{APIKey: cfg.APIKey, Model: cfg.Model}).ListModels(ctx)
	if err != nil {
		c.JSON(http.StatusBadGateway, googleConnectionFailure(err, cfg.APIKey))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"models": models, "defaultModel": ai.DefaultGoogleModel}})
}

func (h *Handler) AdminTestBotConnection(c *gin.Context) {
	draft := map[string]any{}
	if err := c.ShouldBindJSON(&draft); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "code": "invalid_config", "error": "بيانات اختبار الاتصال غير صحيحة"})
		return
	}
	cfg, ok := h.botConnectionConfig(c, draft)
	if !ok {
		return
	}
	provider, err := providers.NewProvider(cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "اختر Google AI Studio كمزوّد الاتصال"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	start := time.Now()
	response, err := provider.Generate(ctx, ai.GenerateRequest{
		Messages:  []ai.Message{{Role: ai.RoleUser, Content: "Reply with only OK."}},
		MaxTokens: 1024,
		OnText:    func(string) {},
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, googleConnectionFailure(err, cfg.APIKey))
		return
	}
	if strings.TrimSpace(response.Content) == "" {
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "code": "empty_response", "error": "Google لم يرجع نصًا في الاختبار. جرّب موديلًا آخر أو أعد المحاولة؛ لم يتم تأكيد نجاح الاتصال."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"ok": true, "provider": provider.Name(), "model": cfg.Model,
		"streamVerified": true, "latencyMs": time.Since(start).Milliseconds(),
		"checkedAt": time.Now().UTC().Format(time.RFC3339),
		"message":   "المفتاح صحيح والاتصال بـ Google ناجح؛ تم استلام رد فعلي عبر البث المستخدم في الشات.",
	}})
}

func googleConnectionFailure(err error, apiKey string) gin.H {
	code := "provider_network"
	message := "تعذر الوصول إلى Google من السيرفر. تحقق من اتصال السيرفر بالإنترنت وأعد الاختبار."
	var providerErr ai.ProviderError
	var networkErr net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &networkErr) && networkErr.Timeout():
		code = "provider_timeout"
		message = "انتهت مهلة انتظار رد Google. أعد الاختبار أو اختر موديل Flash أسرع؛ هذا لا يعني أن المفتاح غير صحيح."
	case errors.As(err, &providerErr):
		code = string(providerErr.Code)
		switch providerErr.Code {
		case ai.ErrorCodeProviderAuth:
			message = "Google رفض المفتاح أو صلاحياته. تأكد من مفتاح Google AI Studio والسماح له باستخدام Gemini API."
		case ai.ErrorCodeProviderModel:
			message = "الموديل المحدد غير متاح. حدّث قائمة الموديلات واختر موديلًا متاحًا لهذا المفتاح."
		case ai.ErrorCodeProviderQuota:
			message = "حصة Google أو رصيد المشروع غير كافٍ. راجع الحصة والفوترة في Google AI Studio ثم أعد الاختبار."
		case ai.ErrorCodeProviderRate:
			message = "Google أوقف الطلب مؤقتًا بسبب كثرة الطلبات. انتظر قليلًا وأعد الاختبار."
		default:
			message = "Google لم يقبل طلب الاختبار. راجع الموديل وإعدادات المشروع ثم أعد المحاولة."
		}
	}
	details := err.Error()
	if apiKey != "" {
		details = strings.ReplaceAll(details, apiKey, "[redacted]")
	}
	return gin.H{"ok": false, "code": code, "error": message, "details": details}
}
