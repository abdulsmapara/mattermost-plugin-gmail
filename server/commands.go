package main

import (
	"fmt"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"google.golang.org/api/gmail/v1"
	"strings"
)

// ExecuteCommand executes the commands registered on getCommand() via RegisterCommand hook
func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	// Obtain base command and its associated action -
	// Split the entered command based on white space (" ")
	arguments := strings.Fields(args.Command)

	// Example "gmail" in command "/gmail"
	baseCommand := arguments[0]

	// Example "connect" in command "/gmail connect"
	action := ""
	if len(arguments) > 1 {
		action = arguments[1]
	}

	// if command not '/gmail', then return
	if baseCommand != "/gmail" {
		return &model.CommandResponse{}, nil
	}

	switch action {
	case "connect":
		return p.handleConnectCommand(c, args)
	case "disconnect":
		return p.handleDisconnectCommand(c, args)
	case "import":
		return p.handleImportCommand(c, args)
	case "":
		return p.handleHelpCommand(c, args)
	case "help":
		return p.handleHelpCommand(c, args)
	default:
		return p.handleInvalidCommand(c, args, action)
	}
}

// handleConnectCommand connects the user with Gmail account
func (p *Plugin) handleConnectCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	// Check if SiteURL is defined in the app
	siteURL := p.API.GetConfig().ServiceSettings.SiteURL
	if siteURL == nil {
		p.sendMessageFromBot(args.ChannelId, args.UserId, true, "Error! Site URL is not defined in the App")
		return &model.CommandResponse{}, nil
	}

	// Send an ephemeral post with the link to connect gmail
	p.sendMessageFromBot(args.ChannelId, args.UserId, true, fmt.Sprintf("[Click here to connect your Gmail account with Mattermost.](%s/plugins/%s/oauth/connect)", *siteURL, manifest.Id))

	return &model.CommandResponse{}, nil
}

// handleDisconnectCommand disconnects the user with Gmail account
func (p *Plugin) handleDisconnectCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	// Send an ephemeral post with the link to disconnect gmail
	p.sendMessageFromBot(args.ChannelId, args.UserId, true, fmt.Sprintf(""))
	return &model.CommandResponse{}, nil
}

// handleHelpCommand posts help about the plugin
func (p *Plugin) handleHelpCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	p.sendMessageFromBot(args.ChannelId, args.UserId, true, helpTextHeader+commonHelpText)
	return &model.CommandResponse{}, nil
}

// handleImportCommand handles the command `/gmail import thread [id]` and `/gmail import mail [id]`
func (p *Plugin) handleImportCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {

	arguments := strings.Fields(args.Command)
	// validate arguments of the command
	if len(arguments) < 3 {
		p.sendMessageFromBot(args.ChannelId, args.UserId, true, "Please use `thread` or `mail` after `/gmail import`. Also provide the ID of thread/mail.")
		return &model.CommandResponse{}, nil
	}
	queryType := arguments[2]
	if queryType != "thread" && queryType != "mail" {
		p.sendMessageFromBot(args.ChannelId, args.UserId, true, "Only `thread` and `mail` are supported after `/gmail import`.")
		return &model.CommandResponse{}, nil
	}
	if len(arguments) < 4 {
		p.sendMessageFromBot(args.ChannelId, args.UserId, true, "Please provide the ID of "+arguments[2])
		return &model.CommandResponse{}, nil
	}
	rfcID := arguments[3]

	gmailID, err := p.getGmailID(args.UserId)
	if err != nil {
		p.sendMessageFromBot(args.ChannelId, args.UserId, true, err.Error())
		return &model.CommandResponse{}, nil
	}

	gmailService, err := p.getGmailService(args.UserId)
	if err != nil {
		p.sendMessageFromBot(args.ChannelId, args.UserId, true, err.Error())
		return &model.CommandResponse{}, nil
	}
	p.API.LogInfo("gmailService created successfully")

	if queryType == "thread" {
		threadID, threadIDErr := p.getThreadID(args.UserId, gmailID, rfcID)
		if threadIDErr != nil {
			p.sendMessageFromBot(args.ChannelId, args.UserId, true, threadIDErr.Error())
			return &model.CommandResponse{}, nil
		}
		thread, threadErr := gmailService.Users.Threads.Get(gmailID, threadID).Format("minimal").Do()
		if threadErr != nil {
			p.sendMessageFromBot(args.ChannelId, args.UserId, true, threadErr.Error())
			return &model.CommandResponse{}, nil
		}
		threadMessages := []*gmail.Message{}
		for _, messageInfo := range thread.Messages {
			message, mailErr := gmailService.Users.Messages.Get(gmailID, messageInfo.Id).Format("raw").Do()
			if mailErr != nil {
				p.sendMessageFromBot(args.ChannelId, args.UserId, true, "Unable to get the thread.")
				return &model.CommandResponse{}, nil
			}
			threadMessages = append(threadMessages, message)
		}
		p.handleMessages(threadMessages, args.ChannelId, args.UserId)

		return &model.CommandResponse{}, nil
	}
	// if queryType == "mail" =>
	// Note that explicit condition check is not required

	messageID, err := p.getMessageID(args.UserId, gmailID, rfcID)
	if err != nil {
		p.sendMessageFromBot(args.ChannelId, args.UserId, true, err.Error())
		p.API.LogInfo(err.Error())
		return &model.CommandResponse{}, nil
	}
	p.API.LogInfo("Extracted Message ID from rfc ID successfully")

	message, err := gmailService.Users.Messages.Get(gmailID, messageID).Format("raw").Do()
	if err != nil {
		p.sendMessageFromBot(args.ChannelId, args.UserId, true, "Unable to get the mail.")
		return &model.CommandResponse{}, nil
	}
	p.API.LogInfo("Message extracted successfully")

	// Message extracted successfully
	p.handleMessages([]*gmail.Message{message}, args.ChannelId, args.UserId)

	return &model.CommandResponse{}, nil
}

// handleInvalidCommand
func (p *Plugin) handleInvalidCommand(c *plugin.Context, args *model.CommandArgs, action string) (*model.CommandResponse, *model.AppError) {
	p.sendMessageFromBot(args.ChannelId, args.UserId, true, "##### Unknown Command: "+action+"\n"+helpTextHeader+commonHelpText)
	return &model.CommandResponse{}, nil
}
