package main

import (
	"errors"
	"fmt"
	"github.com/mattermost/mattermost/server/public/model"
)

// debugPrefix پیشوندی برای لاگ‌های دیباگ
const debugPrefix = "[MuChat]"

// Configuration ساختار تنظیمات پلاگین MuChat
// شامل کلید API، شناسه عامل و وضعیت دیباگ است.
type Configuration struct {
	MuChatApiKey string // کلید API برای احراز هویت با سرویس MuChat
	AgentID      string // شناسه عامل MuChat برای ارسال پیام‌ها
	EnableDebug  bool   // فعال‌سازی حالت دیباگ برای لاگ‌های اضافی
}

// Clone یک کپی از تنظیمات فعلی ایجاد می‌کند.
func (c *Configuration) Clone() *Configuration {
	return &Configuration{
		MuChatApiKey: c.MuChatApiKey,
		AgentID:      c.AgentID,
		EnableDebug:  c.EnableDebug,
	}
}

// IsValid بررسی می‌کند که آیا تنظیمات معتبر هستند یا خیر.
// اگر هر یک از فیلدها خالی باشد، خطای توصیفی بازمی‌گرداند.
func (c *Configuration) IsValid() error {
	if c.MuChatApiKey == "" {
		return errors.New("کلید API نمی‌تواند خالی باشد")
	}
	if c.AgentID == "" {
		return errors.New("شناسه عامل نمی‌تواند خالی باشد")
	}
	return nil
}

// LogDebug یک پیام دیباگ را چاپ می‌کند اگر حالت دیباگ فعال باشد.
func (c *Configuration) LogDebug(message string) {
	if c.EnableDebug {
		fmt.Printf("%s %s\n", debugPrefix, message)
	}
}
