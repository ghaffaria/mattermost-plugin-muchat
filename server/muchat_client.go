package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// MuChatClient ساختار کلاینت برای ارتباط با API سرویس MuChat
// شامل کلید API و کلاینت HTTP است.
type MuChatClient struct {
	apiKey string       // کلید API برای احراز هویت
	http   *http.Client // کلاینت HTTP برای ارسال درخواست‌ها
}

// NewMuChatClient یک نمونه جدید از MuChatClient ایجاد می‌کند.
// apiKey: کلید API برای احراز هویت با سرویس MuChat
func NewMuChatClient(apiKey string) *MuChatClient {
	return &MuChatClient{
		apiKey: apiKey,
		http: &http.Client{
			Timeout: 60 * time.Second, // تنظیم تایم‌اوت ۶۰ ثانیه‌ای
		},
	}
}

// Ask یک سوال را به عامل MuChat ارسال می‌کند و پاسخ را دریافت می‌کند.
// ctx: کانتکست برای مدیریت تایم‌اوت و لغو درخواست
// agentID: شناسه عامل MuChat
// question: سوالی که باید ارسال شود
// stream: اگر true باشد، پاسخ به صورت استریم دریافت می‌شود
func (c *MuChatClient) Ask(ctx context.Context, agentID, question string, stream bool) (io.ReadCloser, error) {
	url := fmt.Sprintf("https://app.mu.chat/api/agents/%s/query", agentID)

	// ساختن بدنه درخواست به صورت JSON
	payload := map[string]interface{}{
		"query":  question,
		"stream": stream,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("خطا در سریال‌سازی JSON: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("خطا در ساخت درخواست HTTP: %w", err)
	}

	// افزودن هدرهای لازم
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("خطا در ارسال درخواست HTTP: %w", err)
	}

	// بررسی کد وضعیت پاسخ
	if resp.StatusCode == http.StatusForbidden {
		return nil, errors.New("دسترسی غیرمجاز: کلید API ممکن است نامعتبر باشد")
	}
	if resp.StatusCode == http.StatusBadRequest {
		return nil, errors.New("درخواست نامعتبر: پارامترهای ارسال شده ممکن است اشتباه باشند")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("خطای غیرمنتظره: کد وضعیت %d", resp.StatusCode)
	}

	// اگر stream فعال باشد، پاسخ به صورت استریم بازگردانده می‌شود
	if stream {
		return resp.Body, nil
	}

	// اگر stream غیرفعال باشد، کل پاسخ خوانده می‌شود
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("خطا در دیکد کردن پاسخ JSON: %w", err)
	}

	// تبدیل پاسخ به یک Reader برای بازگرداندن
	responseBody, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("خطا در سریال‌سازی پاسخ JSON: %w", err)
	}

	return io.NopCloser(bytes.NewReader(responseBody)), nil
}
