// mu-chat-plugin/server/configuration.go
// mu-chat-plugin/server/configuration.go
package main

import (
	"reflect"
	"strings"

	"github.com/pkg/errors"
)

/*
   Configuration مدل کامل تنظیمات قابل‌نمایش در System Console و فیلدهای
   محاسبه‌شده است. فیلدهای public (حرف بزرگ) مستقیماً توسط
   p.API.LoadPluginConfiguration پر می‌شوند.
*/
type Configuration struct {
	/* کلیدهای فعلی */
	MuChatApiKey string
	AgentID      string
	EnableDebug  bool

	/* ──────────────── فیلدهای دسترسی کانال ──────────────── */
	ChannelAccess     string // allow_all | allow_selected | block_selected | block_all
	ChannelAllowList  string // رشتهٔ comma-sep از ChannelID
	ChannelBlockList  string // رشتهٔ comma-sep از ChannelID

	/* ──────────────── فیلدهای دسترسی کاربر ──────────────── */
	UserAccess     string // allow_all | allow_selected | block_selected
	UserAllowList  string // comma-sep UserID
	UserBlockList  string // comma-sep UserID

	/* فیلدهای محاسبه‌شده (هنگام OnConfigurationChange پر می‌شوند) */
	ChannelAllowIDs []string `json:"-"`
	ChannelBlockIDs []string `json:"-"`
	UserAllowIDs    []string `json:"-"`
	UserBlockIDs    []string `json:"-"`
}

/* Clone: deep copy شامل sliceها */
func (c *Configuration) Clone() *Configuration {
	var clone = *c
	clone.ChannelAllowIDs = append([]string(nil), c.ChannelAllowIDs...)
	clone.ChannelBlockIDs = append([]string(nil), c.ChannelBlockIDs...)
	clone.UserAllowIDs = append([]string(nil), c.UserAllowIDs...)
	clone.UserBlockIDs = append([]string(nil), c.UserBlockIDs...)
	return &clone
}

/* ─────────────────────────── دسترسی thread-safe ─────────────────────────── */

func (p *Plugin) getConfiguration() *Configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		return &Configuration{}
	}
	return p.configuration
}

func (p *Plugin) setConfiguration(configuration *Configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	if configuration != nil && p.configuration == configuration {
		if reflect.ValueOf(*configuration).NumField() == 0 {
			return
		}
		panic("setConfiguration called with the existing configuration")
	}
	p.configuration = configuration
}

/* OnConfigurationChange: بارگذاری + محاسبهٔ لیست‌ها */
func (p *Plugin) OnConfigurationChange() error {
	cfg := new(Configuration)
	if err := p.API.LoadPluginConfiguration(cfg); err != nil {
		return errors.Wrap(err, "failed to load plugin configuration")
	}

	// helper to split and trim IDs
	split := func(s string) []string {
		var out []string
		for _, part := range strings.Split(s, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
		return out
	}

	cfg.ChannelAllowIDs = split(cfg.ChannelAllowList)
	cfg.ChannelBlockIDs = split(cfg.ChannelBlockList)
	cfg.UserAllowIDs = split(cfg.UserAllowList)
	cfg.UserBlockIDs = split(cfg.UserBlockList)

	p.setConfiguration(cfg)
	return nil
}
