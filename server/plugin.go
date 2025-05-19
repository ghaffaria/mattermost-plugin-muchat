package main

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	// ── Mattermost SDK
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"

	// ── لایهٔ کمکی داخلی
	"github.com/ghaffaria/mattermost-plugin-starter-template/server/command"
	"github.com/ghaffaria/mattermost-plugin-starter-template/server/store/kvstore"
)

/*───────────────────────────────
   ساختار اصلی پلاگین
───────────────────────────────*/
type Plugin struct {
	plugin.MattermostPlugin

	client        *pluginapi.Client
	kvstore       kvstore.KVStore
	commandClient command.Command
	backgroundJob *cluster.Job

	configurationLock sync.RWMutex
	configuration     *Configuration

	botUserID string
}

/*───────────────────────────────
   OnActivate
───────────────────────────────*/
func (p *Plugin) OnActivate() error {
	p.client = pluginapi.NewClient(p.API, p.Driver)
	p.kvstore = kvstore.NewKVStore(p.client)
	p.commandClient = command.NewCommandHandler(p.client)

	bot := &model.Bot{
		Username:    "muchat",
		DisplayName: "MuChat Bot",
		Description: "Conversational AI powered by MuChat",
	}
	botID, appErr := p.client.Bot.EnsureBot(bot)
	if appErr != nil {
		return errors.Wrap(appErr, "cannot ensure bot")
	}
	p.botUserID = botID

	if err := p.API.RegisterCommand(command.GetCommand()); err != nil {
		return err
	}

	job, err := cluster.Schedule(
		p.API,
		"BackgroundJob",
		cluster.MakeWaitForRoundedInterval(1*time.Hour),
		p.runJob,
	)
	if err != nil {
		return errors.Wrap(err, "schedule background job")
	}
	p.backgroundJob = job
	return nil
}

/*───────────────────────────────
   MessageHasBeenPosted
───────────────────────────────*/
func (p *Plugin) MessageHasBeenPosted(_ *plugin.Context, post *model.Post) {
	// ﴾۱﴿ پیام خودِ Bot را نادیده بگیر
	if post.UserId == p.botUserID {
		return
	}

	// ﴾۲﴿ نوع کانال را بگیر
	channel, chErr := p.API.GetChannel(post.ChannelId)
	if chErr != nil {
		logError(p, chErr, "cannot get channel")
		return
	}
	isDM := channel.Type == model.ChannelTypeDirect

	// ﴾۳﴿ در کانال غیر DM وجود @muchat الزامی است
	if !isDM && !strings.Contains(post.Message, "@muchat") {
		return
	}

	// ﴾۴﴿ متن پرسش
	message := post.Message
	if !isDM {
		message = strings.ReplaceAll(message, "@muchat", "")
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}

	// ﴾۵﴿ تماس با MuChat
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg := p.getConfiguration()
	rc, err := NewMuChatClient(cfg.MuChatApiKey).Ask(ctx, cfg.AgentID, message, false)
	if err != nil {
		logError(p, err, "MuChat request failed")
		return
	}
	defer rc.Close()

	var sb strings.Builder
	buf := make([]byte, 2048)
	for {
		n, rerr := rc.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			logError(p, rerr, "read MuChat response")
			return
		}
	}

	clean := strings.TrimSpace(sb.String())
	if clean == "" {
		clean = "متأسفم، پاسخی دریافت نشد."
	}

	_, _ = p.API.CreatePost(&model.Post{
		UserId:    p.botUserID,
		ChannelId: post.ChannelId,
		RootId:    post.Id,
		Message:   clean,
	})
}
