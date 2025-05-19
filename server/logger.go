package main

import (
	"fmt"
	"log"
)

// logDebug یک پیام دیباگ را لاگ می‌کند اگر حالت دیباگ فعال باشد.
// هر خط لاگ با "[MuChat]" شروع می‌شود.
// p: نمونه‌ای از پلاگین
// msg: پیام دیباگ
// kv: کلید-مقدارهای اضافی برای لاگ
func logDebug(p *Plugin, msg string, kv ...any) {
	if p.configuration != nil && p.configuration.EnableDebug {
		logMessage := fmt.Sprintf("%s %s", debugPrefix, msg)
		if len(kv) > 0 {
			logMessage = fmt.Sprintf("%s | %v", logMessage, kv)
		}
		log.Println(logMessage)
	}
}

// logError یک خطای رخ داده را لاگ می‌کند.
// هر خط لاگ با "[MuChat]" شروع می‌شود.
// p: نمونه‌ای از پلاگین
// err: خطای رخ داده
// kv: کلید-مقدارهای اضافی برای لاگ
func logError(p *Plugin, err error, kv ...any) {
	logMessage := fmt.Sprintf("%s خطا: %v", debugPrefix, err)
	if len(kv) > 0 {
		logMessage = fmt.Sprintf("%s | %v", logMessage, kv)
	}
	log.Println(logMessage)
}
