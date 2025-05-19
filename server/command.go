package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

// GetCommand تعریف دستور /mu برای پلاگین MuChat را بازمی‌گرداند.
func GetCommand() *model.Command {
	return &model.Command{
		Trigger:          "mu",
		AutoComplete:     true,
		AutoCompleteDesc: "ارسال پیام به عامل MuChat",
		AutoCompleteHint: "[پیام شما]",
	}
}

// ExecuteCommand اجرای دستور /mu را مدیریت می‌کند.
// ctx: کانتکست برای مدیریت تایم‌اوت و لغو درخواست
// args: آرگومان‌های دستور شامل متن پیام
func (p *Plugin) ExecuteCommand(ctx context.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if strings.TrimSpace(args.Command) == "/mu" {
		return &model.CommandResponse{}, &model.AppError{
			Message: "لطفاً یک پیام وارد کنید.",
		}
	}

	message := strings.TrimPrefix(args.Command, "/mu ")
	if message == "" {
		return &model.CommandResponse{}, &model.AppError{
			Message: "پیام نمی‌تواند خالی باشد.",
		}
	}

	// ارسال پیام "در حال تایپ..."
	post := &model.Post{
		ChannelId: args.ChannelId,
		UserId:    args.UserId,
		Message:   "در حال تایپ...",
	}
	createdPost, appErr := p.API.CreatePost(post)
	if appErr != nil {
		return nil, appErr
	}

	// ارسال پیام به MuChatClient
	client := NewMuChatClient(p.configuration.MuChatApiKey)
	response, err := client.Ask(ctx, p.configuration.AgentID, message, true)
	if err != nil {
		logError(p, err, "خطا در ارسال پیام به MuChat")
		return nil, &model.AppError{
			Message: fmt.Sprintf("خطا در ارتباط با MuChat: %v", err),
		}
	}
	defer response.Close()

	// دریافت پاسخ به صورت استریم
	var responseText strings.Builder
	buf := make([]byte, 1024)
	for {
		bytesRead, readErr := response.Read(buf)
		if bytesRead > 0 {
			chunk := string(buf[:bytesRead])
			responseText.WriteString(chunk)

			// به‌روزرسانی پیام در حال تایپ
			createdPost.Message = responseText.String()
			if _, updateErr := p.API.UpdatePost(createdPost); updateErr != nil {
				logError(p, updateErr, "خطا در به‌روزرسانی پیام")
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			logError(p, readErr, "خطا در خواندن پاسخ استریم")
			return nil, &model.AppError{
				Message: fmt.Sprintf("خطا در خواندن پاسخ: %v", readErr),
			}
		}
	}

	// به‌روزرسانی پیام نهایی
	createdPost.Message = responseText.String()
	if _, updateErr := p.API.UpdatePost(createdPost); updateErr != nil {
		logError(p, updateErr, "خطا در به‌روزرسانی پیام نهایی")
	}

	logDebug(p, "دستور /mu با موفقیت اجرا شد.", "پیام", message)
	return &model.CommandResponse{}, nil
}
