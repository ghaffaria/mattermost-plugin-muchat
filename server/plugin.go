// mu-chat-plugin/server/plugin.go
// mu-chat-plugin/server/plugin.go
package main

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	// Mattermost SDK
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"

	// internal helpers
	"github.com/ghaffaria/mattermost-plugin-starter-template/server/command"
	"github.com/ghaffaria/mattermost-plugin-starter-template/server/store/kvstore"
)

/*───────────────────────────────
   Plugin struct
───────────────────────────────*/
type Plugin struct {
	plugin.MattermostPlugin

	client        *pluginapi.Client
	kvstore       kvstore.KVStore
	commandClient command.Command
	backgroundJob *cluster.Job

	configurationLock sync.RWMutex
	configuration     *Configuration

	botUserID   string
	botUsername string
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
	p.botUsername = bot.Username

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
   Helpers
───────────────────────────────*/
func contains(list []string, id string) bool {
	for _, v := range list {
		if v == id {
			return true
		}
	}
	return false
}

func isAllowed(id, mode string, allow, block []string, isChannel bool) bool {
	switch mode {
	case "block_all":
		if isChannel {
			return false
		}
		return true
	case "allow_selected":
		return contains(allow, id)
	case "block_selected":
		return !contains(block, id)
	default: // allow_all
		return true
	}
}

/*───────────────────────────────
   MessageHasBeenPosted
───────────────────────────────*/
func (p *Plugin) MessageHasBeenPosted(_ *plugin.Context, post *model.Post) {
	// ignore messages from the bot itself
	if post.UserId == p.botUserID {
		return
	}

	cfg := p.getConfiguration()

	// fetch channel information
	channel, chErr := p.API.GetChannel(post.ChannelId)
	if chErr != nil {
		logError(p, chErr, "cannot get channel")
		return
	}

	// ignore if bot is not a member of the channel
	if _, err := p.API.GetChannelMember(channel.Id, p.botUserID); err != nil {
		return
	}

	// access control checks
	if !isAllowed(channel.Id, cfg.ChannelAccess, cfg.ChannelAllowIDs, cfg.ChannelBlockIDs, true) {
		return
	}
	if !isAllowed(post.UserId, cfg.UserAccess, cfg.UserAllowIDs, cfg.UserBlockIDs, false) {
		return
	}

	isDM := channel.Type == model.ChannelTypeDirect

	// mention logic
	mentioned := strings.Contains(post.Message, "@"+p.botUsername) ||
		strings.Contains(post.Message, "<@"+p.botUserID+">")

	if !isDM && !mentioned {
		return
	}

	// strip mention for non-DM messages
	message := post.Message
	if !isDM {
		message = strings.ReplaceAll(message, "@"+p.botUsername, "")
		message = strings.ReplaceAll(message, "<@"+p.botUserID+">", "")
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}

	// call MuChat
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

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

	reply := strings.TrimSpace(sb.String())
	if reply == "" {
		reply = "متأسفم، پاسخی دریافت نشد."
	}

	_, _ = p.API.CreatePost(&model.Post{
		UserId:    p.botUserID,
		ChannelId: post.ChannelId,
		RootId:    post.Id,
		Message:   reply,
	})
}
