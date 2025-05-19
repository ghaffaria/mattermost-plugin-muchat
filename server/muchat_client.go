package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

/*
   ────────────────────────────────────────────────────────
   ساختار و سازندهٔ کلاینت
*/

type MuChatClient struct {
	apiKey string
	http   *http.Client
}

// NewMuChatClient یک نمونهٔ جدید از کلاینت می‌سازد.
func NewMuChatClient(apiKey string) *MuChatClient {
	return &MuChatClient{
		apiKey: apiKey,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

/*
   ────────────────────────────────────────────────────────
   مدل پاسخ موردنیاز
*/
type muChatResponse struct {
	Answer string `json:"answer"`
}

/*
Ask سؤال را به MuChat می‌فرستد و فقط **متن پاسخ** را برمی‌گرداند.

اگر `stream` false باشد:
	• بدنهٔ JSON را کامل می‌خواند
	• فیلد answer را استخراج می‌کند
	• همان متن را به صورت Reader برمی‌گرداند

اگر `stream` true باشد:
	• رویدادهای SSE را خط‌به‌خط می‌خواند
	• فقط رویداد `answer` را Unmarshal می‌کند
	• توکن‌های پاسخ را به‌‌صورت پیوسته در یک Pipe می‌نویسد
*/
func (c *MuChatClient) Ask(ctx context.Context, agentID, query string, stream bool) (io.ReadCloser, error) {
	url := fmt.Sprintf("https://app.mu.chat/api/agents/%s/query", agentID)

	payload, _ := json.Marshal(map[string]interface{}{
		"query":  query,
		"stream": stream,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("ساخت درخواست HTTP شکست خورد: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ارسال HTTP شکست خورد: %w", err)
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, errors.New("دسترسی غیرمجاز: کلید API نامعتبر است")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("خطای غیرمنتظره: %d", resp.StatusCode)
	}

	/*──────────── حالت استریم ────────────*/
	if stream {
		pr, pw := io.Pipe()
		go func() {
			defer resp.Body.Close()
			defer pw.Close()

			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "data:") {
					var r muChatResponse
					if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data:")), &r); err == nil {
						if _, werr := pw.Write([]byte(r.Answer)); werr != nil {
							return
						}
					}
				}
			}
		}()
		return pr, nil
	}

	/*──────────── حالت غیر استریم ────────────*/
	defer resp.Body.Close()
	var r muChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("دیکد JSON شکست خورد: %w", err)
	}
	return io.NopCloser(strings.NewReader(r.Answer)), nil
}
