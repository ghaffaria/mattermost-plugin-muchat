package main

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	// بسته‌های عمومی Mattermost
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"

	// لایهٔ کمکی داخلی
	"github.com/ghaffaria/mattermost-plugin-starter-template/server/command"
	"github.com/ghaffaria/mattermost-plugin-starter-template/server/store/kvstore"
)

/*───────────────────────────────
   ساختار اصلی پلاگین
───────────────────────────────*/
type Plugin struct {
	plugin.MattermostPlugin

	// وابستگی‌های کمکی
	client        *pluginapi.Client
	kvstore       kvstore.KVStore
	commandClient command.Command
	backgroundJob *cluster.Job

	// پیکربندی
	configurationLock sync.RWMutex
	configuration     *Configuration

	// Bot
	botUserID string
}

/*───────────────────────────────
   OnActivate
───────────────────────────────*/
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

	// 3) Job پس‌زمینهٔ نمونه (هر ساعت)
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

/*───────────────────────────────
   OnDeactivate
───────────────────────────────*/
func (p *Plugin) OnDeactivate() error {
	if p.backgroundJob != nil {
		if err := p.backgroundJob.Close(); err != nil {
			p.API.LogError("Failed to close background job", "err", err)
		}
	}
	return nil
}

/*───────────────────────────────
   MessageHasBeenPosted
───────────────────────────────*/
func (p *Plugin) MessageHasBeenPosted(_ *plugin.Context, post *model.Post) {
	// پیام خود Bot
	if post.UserId == p.botUserID {
		return
	}
	// منشن
	if !strings.Contains(post.Message, "@muchat") {
		return
	}

	// پاک‌کردن منشن
	message := strings.ReplaceAll(post.Message, "@muchat", "")

	// تماس با MuChat
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg := p.getConfiguration()
	client := NewMuChatClient(cfg.MuChatApiKey)

	reader, err := client.Ask(ctx, cfg.AgentID, message, false)
	if err != nil {
		logError(p, err, "خطا در تماس با MuChat")
		return
	}
	defer reader.Close()

	// خواندن پاسخ
	var sb strings.Builder
	buf := make([]byte, 2048)
	for {
		n, rerr := reader.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			logError(p, rerr, "خطا در خواندن پاسخ")
			return
		}
	}

	// تمیزکاری متن خروجی
	clean := strings.TrimSpace(sb.String())
	clean = strings.ReplaceAll(clean, "\r\n", "\n")
	clean = strings.ReplaceAll(clean, "\n\n", "\n")
	clean = strings.TrimLeft(clean, ". \n\t")

	// ارسال پست پاسخ
	reply := &model.Post{
		UserId:    p.botUserID,
		ChannelId: post.ChannelId,
		RootId:    post.Id,
		Message:   clean,
	}
	if _, err = p.API.CreatePost(reply); err != nil {
		logError(p, err, "خطا در ارسال پاسخ MuChat")
	}
}
