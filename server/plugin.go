package main

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	// 📦  بسته‌های عمومی پلاگین
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"

	// 📦  لایهٔ کمکی داخلی
	"github.com/ghaffaria/mattermost-plugin-starter-template/server/command"
	"github.com/ghaffaria/mattermost-plugin-starter-template/server/store/kvstore"
)

/*
	ساختار اصلی پلاگین – تمام منطق سرور اینجا نگه‌داری می‌شود.
*/
type Plugin struct {
	plugin.MattermostPlugin

	// ────────────────────────── وابستگی‌های کمکی
	client        *pluginapi.Client
	kvstore       kvstore.KVStore
	commandClient command.Command
	backgroundJob *cluster.Job

	// ────────────────────────── پیکربندی
	configurationLock sync.RWMutex
	configuration     *Configuration

	// ────────────────────────── Bot
	botUserID string
}

/*
	OnActivate هنگام فعال‌سازی پلاگین:

	1) ساخت Bot (اگر قبلاً نبوده)
	2) ثبت دستور /mu
	3) برنامه‌ریزی Job پس‌زمینه (نمونه)
*/
func (p *Plugin) OnActivate() error {
	// کلاینت کمکی
	p.client = pluginapi.NewClient(p.API, p.Driver)
	p.kvstore = kvstore.NewKVStore(p.client)
	p.commandClient = command.NewCommandHandler(p.client)

	// 1) اطمینان از وجود Bot
	bot := &model.Bot{
		Username:    "muchat",
		DisplayName: "MuChat Bot",
		Description: "Conversational AI powered by MuChat",
	}
	botID, appErr := p.client.Bot.EnsureBot(bot)
	if appErr != nil {
		return errors.Wrap(appErr, "cannot ensure bot account")
	}
	p.botUserID = botID

	// 2) ثبت Slash-Command
	if err := p.API.RegisterCommand(command.GetCommand()); err != nil {
		logError(p, err, "خطا در ثبت دستور /mu")
		return err
	}

	// 3) Job نمونهٔ پس‌زمینه (هر ساعت)
	job, err := cluster.Schedule(
		p.API,
		"BackgroundJob",
		cluster.MakeWaitForRoundedInterval(1*time.Hour),
		p.runJob,
	)
	if err != nil {
		return errors.Wrap(err, "failed to schedule background job")
	}
	p.backgroundJob = job

	logDebug(p, "MuChat plugin activated ✓")
	return nil
}

/*
	OnDeactivate در زمان غیرفعال‌سازی پلاگین
*/
func (p *Plugin) OnDeactivate() error {
	if p.backgroundJob != nil {
		if err := p.backgroundJob.Close(); err != nil {
			p.API.LogError("Failed to close background job", "err", err)
		}
	}
	return nil
}

/*
	MessageHasBeenPosted:
	اگر @muchat در پیام باشد، متن را به MuChat می‌فرستد و پاسخ را ریپلای می‌کند.
*/
func (p *Plugin) MessageHasBeenPosted(_ *plugin.Context, post *model.Post) {
	// ۱) پیام خودِ Bot را نادیده بگیر
	if post.UserId == p.botUserID {
		return
	}
	// ۲) بررسی منشن
	if !strings.Contains(post.Message, "@muchat") {
		return
	}

	// ۳) پاک‌کردن منشن از متن
	message := strings.ReplaceAll(post.Message, "@muchat", "")

	// ۴) تماس با MuChat
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg := p.getConfiguration()
	client := NewMuChatClient(cfg.MuChatApiKey)

	stream, err := client.Ask(ctx, cfg.AgentID, message, true)
	if err != nil {
		logError(p, err, "خطا در تماس با MuChat")
		return
	}
	defer stream.Close()

	// ۵) خواندن استریم پاسخ
	var sb strings.Builder
	buf := make([]byte, 2048)
	for {
		n, rerr := stream.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			logError(p, rerr, "خطا در خواندن پاسخ استریم")
			return
		}
	}

	// ۶) ارسال ریپلای در همان رشته
	reply := &model.Post{
		UserId:    p.botUserID,
		ChannelId: post.ChannelId,
		RootId:    post.Id,
		Message:   sb.String(),
	}
	if _, err = p.API.CreatePost(reply); err != nil {
		logError(p, err, "خطا در ارسال پاسخ MuChat")
	}
}
