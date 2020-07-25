package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	// "github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	accessAPI "google.golang.org/api/oauth2/v2" // Package oauth2 provides access to the Google OAuth2 API
	"google.golang.org/api/option"
)

// CreateBotDMPost creates a post as gmail bot to the user directly
func (p *Plugin) CreateBotDMPost(userID, message string) error {
	return p.sendMessageFromBot("", userID, false, message)
}

// sendMessageFromBot can create a regular or ephemeral post on Channel or on DM from BOT.
// 1. "For DM Reg post" : [0]channelID, [X]userID, [0]isEphemeralPost.
// 2. "For DM Eph post" : [0]channelID, [X]userID, [X]isEphemeralPost.
// 3. "For Ch Reg post" : [X]channelID, [0]userID, [0]isEphemeralPost.
// 4. "For Ch Eph post" : [X]channelID, [X]userID, [X]isEphemeralPost.
func (p *Plugin) sendMessageFromBot(_channelID string, userID string, isEphemeralPost bool, message string) error {
	var channelID string = _channelID

	// If its nil then get the DM channel of bot and user
	if len(channelID) == 0 {
		if len(userID) == 0 {
			return errors.New("User and Channel ID both are undefined")
		}

		// Get the Bot Direct Message channel
		directChannel, err := p.API.GetDirectChannel(userID, p.gmailBotID)
		if err != nil {
			return err
		}

		channelID = directChannel.Id
	}

	// Construct the Post message
	post := &model.Post{
		UserId:    p.gmailBotID,
		ChannelId: channelID,
		Message:   message,
	}

	if isEphemeralPost == true {
		p.API.SendEphemeralPost(userID, post)
		return nil
	}

	p.API.CreatePost(post)
	return nil

}

func (p *Plugin) getOAuthConfig() *oauth2.Config {
	config := p.API.GetConfig()
	clientID := p.getConfiguration().GmailOAuthClientID
	clientSecret := p.getConfiguration().GmailOAuthSecret

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  fmt.Sprintf("%s/plugins/%s/oauth/complete", *config.ServiceSettings.SiteURL, manifest.Id),
		Scopes: []string{
			emailScope,
			gmail.MailGoogleComScope,
		},
	}
}

// getGmailService retrieves the token stored in database and then generates a gmail service
func (p *Plugin) getGmailService(userID string) (*gmail.Service, error) {
	var token oauth2.Token

	tokenInByte, appErr := p.API.KVGet(userID + "gmailToken")
	if appErr != nil {
		return nil, errors.New(appErr.DetailedError)
	}

	json.Unmarshal(tokenInByte, &token)
	config := p.getOAuthConfig()
	ctx := context.Background()
	tokenSource := config.TokenSource(ctx, &token)
	gmailService, err := gmail.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, err
	}

	return gmailService, nil
}

// getOAuthService generates OAuth Service
func (p *Plugin) getOAuthService(userID string) (*accessAPI.Service, error) {
	var token oauth2.Token

	tokenInByte, appErr := p.API.KVGet(userID + "gmailToken")
	if appErr != nil {
		return nil, errors.New(appErr.DetailedError)
	}
	json.Unmarshal(tokenInByte, &token)
	ctx := context.Background()
	config := p.getOAuthConfig()
	tokenSource := config.TokenSource(ctx, &token)
	oauth2Service, err := accessAPI.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, err
	}

	return oauth2Service, nil
}

// getGmailID retrieves the gmail ID of the user
func (p *Plugin) getGmailID(userID string) (string, error) {
	oauth2Service, err := p.getOAuthService(userID)
	if err != nil {
		return "", err
	}
	userInfo, err := oauth2Service.Userinfo.Get().Do()
	if err != nil {
		return "", err
	}
	return userInfo.Email, nil
}

// getThreadID generates ID of thread from rfcID of the mail in the thread
func (p *Plugin) getThreadID(userID string, gmailID string, rfcID string) (string, error) {
	gmailService, err := p.getGmailService(userID)
	if err != nil {
		return "", err
	}
	listResponse, err := gmailService.Users.Messages.List(gmailID).Q("rfc822msgid:" + rfcID).Do()
	if err != nil {
		return "", err
	}

	return listResponse.Messages[0].ThreadId, nil
}

// getMessageID generates ID of mail/message from rfcID of the mail/message
func (p *Plugin) getMessageID(userID string, gmailID string, rfcID string) (string, error) {
	gmailService, err := p.getGmailService(userID)
	if err != nil {
		return "", err
	}
	listResponse, err := gmailService.Users.Messages.List(gmailID).Q("rfc822msgid:" + rfcID).Do()
	if err != nil {
		return "", err
	}

	return listResponse.Messages[0].Id, nil
}

func (p *Plugin) decodeBase64URL(urlInBase64 string) (string, error) {
	data := strings.Replace(urlInBase64, "-", "+", -1) // 62nd char of encoding
	data = strings.Replace(data, "_", "/", -1)         // 63rd char of encoding

	switch len(data) % 4 { // Pad with trailing '='s
	case 0: // no padding
	case 2:
		data += "==" // 2 pad chars
	case 3:
		data += "=" // 1 pad char
	default:
		fmt.Println("ERROR OCCURRED")
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}

	return string(decoded), nil
}

func (p *Plugin) getMessageDetails(message string) (string, string) {
	if len(message) == 0 {
		return "", ""
	}
	// parse message line by line to find "Subject", "From", "To"
	// split on new line character
	linesInMessage := strings.Split(message, "\n")
	subject := ""
	boundary := ""
	body := ""
	contentTransferEncoding := ""
	bodyBegins := false
	for _, line := range linesInMessage {

		if len(strings.TrimSpace(line)) == 0 {
			continue
		}

		if strings.HasPrefix(line, "Subject") {
			subject = line
		} else if strings.Contains(line, "Content-Type: multipart/alternative; boundary=") {
			boundary = strings.Split(line, "=")[1]
			boundary = strings.ReplaceAll(boundary, "\"", "")
		} else if strings.HasPrefix(line, "--"+boundary) {
			if bodyBegins {
				bodyBegins = false
				break
			}
			bodyBegins = true
		} else if bodyBegins {
			if strings.HasPrefix(line, "Content-Transfer-Encoding:") {
				contentTransferEncoding = strings.Split(line, ":")[1]
				continue
			}
			if strings.HasPrefix(line, "Content-Type:") {
				continue
			}
			body += line
		}
	}
	finalBody := body
	if strings.TrimSpace(contentTransferEncoding) == "base64" {
		finalBody, _ = p.decodeBase64URL(body)
	}

	return subject, finalBody
}
