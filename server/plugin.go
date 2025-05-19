package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pardisco/mattermost-plugin-muchat/server/command"
	"github.com/pardisco/mattermost-plugin-muchat/server/store/kvstore"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
	"github.com/pkg/errors"
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// kvstore is the client used to read/write KV records for this plugin.
	kvstore kvstore.KVStore

	// client is the Mattermost server API client.
	client *pluginapi.Client

	// commandClient is the client used to register and execute slash commands.
	commandClient command.Command

	backgroundJob *cluster.Job

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	// configuration stores the plugin's configuration.
	configLock sync.RWMutex
}

// OnConfigurationChange بارگذاری و اعتبارسنجی تنظیمات جدید را مدیریت می‌کند.
func (p *Plugin) OnConfigurationChange() error {
	var config Configuration
	if err := p.API.LoadPluginConfiguration(&config); err != nil {
		logError(p, err, "خطا در بارگذاری تنظیمات پلاگین")
		return err
	}

	if err := config.IsValid(); err != nil {
		logError(p, err, "تنظیمات نامعتبر است")
		return err
	}

	p.configLock.Lock()
	p.configuration = &config
	p.configLock.Unlock()

	logDebug(p, "تنظیمات پلاگین با موفقیت به‌روزرسانی شد.")
	return nil
}

// OnActivate ثبت دستور /mu را مدیریت می‌کند.
func (p *Plugin) OnActivate() error {
	p.client = pluginapi.NewClient(p.API, p.Driver)

	p.kvstore = kvstore.NewKVStore(p.client)

	p.commandClient = command.NewCommandHandler(p.client)

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

	if err := p.API.RegisterCommand(command.GetCommand()); err != nil {
		logError(p, err, "خطا در ثبت دستور /mu")
		return err
	}

	logDebug(p, "پلاگین با موفقیت فعال شد.")
	return nil
}

// OnDeactivate is invoked when the plugin is deactivated.
func (p *Plugin) OnDeactivate() error {
	if p.backgroundJob != nil {
		if err := p.backgroundJob.Close(); err != nil {
			p.API.LogError("Failed to close background job", "err", err)
		}
	}
	return nil
}

// This will execute the commands that were registered in the NewCommandHandler function.
func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	response, err := p.commandClient.Handle(args)
	if err != nil {
		return nil, model.NewAppError("ExecuteCommand", "plugin.command.execute_command.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return response, nil
}

// MessageHasBeenPosted پیام‌های ارسال‌شده را بررسی می‌کند.
// اگر پیام شامل @muchat باشد، آن را به MuChat ارسال می‌کند.
func (p *Plugin) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
	if !strings.Contains(post.Message, "@muchat") {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	message := strings.ReplaceAll(post.Message, "@muchat", "")
	client := NewMuChatClient(p.configuration.MuChatApiKey)
	response, err := client.Ask(ctx, p.configuration.AgentID, message, true)
	if err != nil {
		logError(p, err, "خطا در ارسال پیام به MuChat")
		return
	}
	defer response.Close()

	var responseText strings.Builder
	buf := make([]byte, 1024)
	for {
		bytesRead, readErr := response.Read(buf)
		if bytesRead > 0 {
			chunk := string(buf[:bytesRead])
			responseText.WriteString(chunk)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			logError(p, readErr, "خطا در خواندن پاسخ استریم")
			return
		}
	}

	reply := &model.Post{
		ChannelId: post.ChannelId,
		RootId:    post.Id,
		Message:   responseText.String(),
	}
	if _, err := p.API.CreatePost(reply); err != nil {
		logError(p, err, "خطا در ارسال پاسخ MuChat")
	}
}

// See https://developers.mattermost.com/extend/plugins/server/reference/
