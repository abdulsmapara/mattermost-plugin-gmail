package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"strconv"
	"strings"
	// "github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/DusanKasan/parsemail"
	html2markdown "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	accessAPI "google.golang.org/api/oauth2/v2" // Package oauth2 provides access to the Google OAuth2 API
	"google.golang.org/api/option"
)

// CreateBotDMPost creates a post as gmail bot to the user directly
func (p *Plugin) CreateBotDMPost(userID, message string) (string, error) {
	return p.sendMessageFromBot("", userID, false, message)
}

// sendMessageFromBot can create a regular or ephemeral post on Channel or on DM from BOT.
// 1. "For DM Reg post" : [0]channelID, [X]userID, [0]isEphemeralPost.
// 2. "For DM Eph post" : [0]channelID, [X]userID, [X]isEphemeralPost.
// 3. "For Ch Reg post" : [X]channelID, [0]userID, [0]isEphemeralPost.
// 4. "For Ch Eph post" : [X]channelID, [X]userID, [X]isEphemeralPost.
func (p *Plugin) sendMessageFromBot(_channelID string, userID string, isEphemeralPost bool, message string) (string, error) {
	var channelID string = _channelID

	// If its nil then get the DM channel of bot and user
	if len(channelID) == 0 {
		if len(userID) == 0 {
			return "", errors.New("User and Channel ID both are undefined")
		}

		// Get the Bot Direct Message channel
		directChannel, err := p.API.GetDirectChannel(userID, p.gmailBotID)
		if err != nil {
			return "", err
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
		postInfo := p.API.SendEphemeralPost(userID, post)
		return postInfo.Id, nil
	}

	postInfo, err := p.API.CreatePost(post)
	if err != nil {
		return "", err
	}
	return postInfo.Id, nil

}

func (p *Plugin) checkIfConnected(userID string) bool {
	accessTokenInBytes, err := p.API.KVGet(userID + "gmailToken")
	if err != nil || accessTokenInBytes == nil {
		return false
	}
	return true
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
	if appErr != nil || tokenInByte == nil {
		p.API.LogError("Error occured while getting gmail token", "err", appErr)
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
	gmailID, kvErr := p.API.KVGet(userID + "gmailID")
	if kvErr == nil && gmailID != nil {
		return string(gmailID), nil
	}
	oauth2Service, err := p.getOAuthService(userID)
	if err != nil {
		return "", err
	}
	userInfo, err := oauth2Service.Userinfo.Get().Do()
	if err != nil || userInfo == nil {
		return "", err
	}
	p.API.KVSet(userID+"gmailID", []byte(userInfo.Email))
	return userInfo.Email, nil
}

// addUserForGmail adds a user connected with the given Gmail ID
func (p *Plugin) addUserForGmail(gmailID string, userID string) error {
	p.API.LogInfo("Adding user with userID: " + userID + " for gmailID: " + gmailID)

	userIDs, err := p.getUsersForGmail(gmailID)
	if err != nil {
		p.API.LogError("Error occured while fetching users connected to a given Gmail ID", "err", err.Error())
		return err
	}
	if userIDs == nil {
		userIDs = []string{}
	}
	userIDs = append(userIDs, userID)

	p.API.KVSet(gmailID+"users", []byte(strings.Join(userIDs, ",")))
	p.API.LogInfo("User added successfully for the gmail ID")

	return nil
}

// removeUserForGmail removes user connected with given Gmail ID
func (p *Plugin) removeUserForGmail(gmailID string, userID string) error {
	userIDs, err := p.getUsersForGmail(gmailID)

	if err != nil {
		p.API.LogError("Error occured while fetching users connected to a given Gmail ID", "err", err.Error())
		return err
	}
	if userIDs == nil {
		p.API.LogError("No user found for gmail ID: " + gmailID)
		return nil
	}
	updatedUserIDs := []string{}

	// TODO: OPTIMIZATION Use set instead of array for storing/updating user IDs
	for _, existingUserID := range userIDs {
		if existingUserID != userID {
			updatedUserIDs = append(updatedUserIDs, existingUserID)
		}
	}

	p.API.KVSet(gmailID+"users", []byte(strings.Join(updatedUserIDs, ",")))
	return nil
}

// onboardUser onboards user to the plugin when connected to a Gmail account
func (p *Plugin) onboardUser(userID string, tokenJSON []byte) error {

	err := p.API.KVSet(userID+"gmailToken", tokenJSON)
	if err != nil {
		p.API.LogError("Error in setting gmail token", "err", err.Error())
		return err
	}

	gmailID, gmailErr := p.getGmailID(userID)
	if gmailErr != nil {
		p.API.LogError("Error in getting gmail ID for the user with user ID: "+userID, "err", gmailErr.Error())
		return gmailErr
	}

	err = p.API.KVSet(userID+"gmailID", []byte(gmailID))
	if err != nil {
		p.API.LogError("Error in setting gmail ID as "+gmailID+" for the user with user ID: "+userID, "err", err.Error())
		return err
	}

	gmailErr = p.addUserForGmail(gmailID, userID)
	if gmailErr != nil {
		p.API.LogError("Error in adding user with user ID: "+userID+" to list of users connected to gmail ID: "+gmailID, "err", gmailErr.Error())
		return gmailErr
	}

	labelErr := p.subscribeToLabels(userID, gmailID, p.getSupportedLabels())
	if labelErr != nil {
		p.API.LogError("Error in subscribing user with user ID: "+userID+" to all supported labels", "err", labelErr.Error())
		return labelErr
	}

	return nil
}

// offboardUser off boards user from the plugin when disconnected from Gmail
func (p *Plugin) offboardUser(userID string) error {
	p.API.LogInfo("Offboarding user with userID: " + userID)

	gmailID, _ := p.getGmailID(userID)

	err := p.removeUserForGmail(gmailID, userID)
	if err != nil {
		return err
	}

	p.API.KVDelete(userID + "subscriptions")

	p.API.KVDelete(userID + "gmailID")

	p.API.KVDelete(userID + "gmailToken")

	p.API.LogInfo("Offboarding successfully completed for the user")

	return nil
}

// getUsersForGmail returns array of user IDs connected with the given Gmail ID
func (p *Plugin) getUsersForGmail(gmailID string) ([]string, error) {
	users, err := p.API.KVGet(gmailID + "users")
	if err != nil {
		return nil, err
	}
	if users == nil {
		return []string{}, nil
	}
	return strings.Split(string(users), ","), nil
}

// updateSubscriptionsOfUser updates subscriptions of the user
func (p *Plugin) updateSubscriptionsOfUser(userID string, labelIDs []string) *model.AppError {
	return p.API.KVSet(userID+"subscriptions", []byte(strings.Join(labelIDs, ",")))
}

// getSubscriptionsOfUser returns subscriptions of the user
func (p *Plugin) getSubscriptionsOfUser(userID string) ([]string, error) {
	subscriptions, err := p.API.KVGet(userID + "subscriptions")
	return strings.Split(string(subscriptions), ","), err
}

// removeAllSubscriptionsOfUser
func (p *Plugin) removeAllSubscriptionsOfUser(userID string) error {
	return p.API.KVDelete(userID + "subscriptions")
}

// updateHistoryIDForGmail updates historyID of the user
func (p *Plugin) updateHistoryIDForUser(historyID uint64, userID string) *model.AppError {
	return p.API.KVSet(userID+"historyID", []byte(strconv.Itoa(int(historyID))))
}

// getHistoryIDForGmail returns history ID of the user
func (p *Plugin) getHistoryIDForUser(userID string) (uint64, error) {
	historyID, err := p.API.KVGet(userID + "historyID")
	if err == nil {
		histID, err := strconv.Atoi(string(historyID))
		return uint64(histID), err
	}
	return uint64(0), err
}

// getThreadID generates ID of thread from rfcID of the mail in the thread
func (p *Plugin) getThreadID(userID string, gmailID string, rfcID string) (string, error) {
	gmailService, err := p.getGmailService(userID)
	if err != nil {
		return "", err
	}
	listCall := gmailService.Users.Messages.List(gmailID).Q("rfc822msgid:" + rfcID)
	listResponse, err := listCall.Do()
	if err != nil {
		return "", err
	}
	if len(listResponse.Messages) != 1 {
		return "", errors.New("Invalid ID. Please provide ID of some mail in the thread")
	}
	return listResponse.Messages[0].ThreadId, nil
}

// getMessageID generates ID of mail/message from rfcID of the mail/message
func (p *Plugin) getMessageID(userID string, gmailID string, rfcID string) (string, error) {
	gmailService, err := p.getGmailService(userID)
	if err != nil {
		return "", err
	}
	listCall := gmailService.Users.Messages.List(gmailID).Q("rfc822msgid:" + rfcID)
	listResponse, err := listCall.Do()
	if err != nil {
		return "", err
	}
	if len(listResponse.Messages) != 1 {
		return "", errors.New("Invalid ID. Please provide a valid mail ID")
	}
	return listResponse.Messages[0].Id, nil
}

func (p *Plugin) decodeBase64URL(urlInBase64 string) (string, error) {
	data := strings.Replace(urlInBase64, "-", "+", -1) // 62nd char of encoding
	data = strings.Replace(data, "_", "/", -1)         // 63rd char of encoding

	switch len(data) % 4 { // Pad with trailing '='s
	case 0: // no padding
		break
	case 2:
		data += "==" // 2 pad chars
		break
	case 3:
		data += "=" // 1 pad char
		break
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		fmt.Println(err.Error())
		return "", err
	}

	return string(decoded), nil
}

func (p *Plugin) parseMessage(message string) (string, string, string, string, string, []parsemail.Attachment, error) {
	// Use parser for email
	reader := strings.NewReader(message)

	email, err := parsemail.Parse(reader) // returns Email struct and error
	if err != nil {
		// return details from self parsed message
		p.API.LogError("Error in using parsemail package", "err", err.Error())
		return "", "", "", "", "", nil, err
	}
	year, month, day := email.Date.Date()
	date := fmt.Sprintf("%v %v, %v", month, day, year)

	emailFrom := email.From
	fromNames := ""
	totalNames := len(emailFrom)
	for fromIndex, fromDetails := range emailFrom {
		if fromDetails.Name != "" {
			fromNames += fromDetails.Name + " "
			if fromIndex != (totalNames - 1) {
				fromNames += ","
			}
		}
	}
	attachments := []parsemail.Attachment{}
	// Attachments
	for _, attachment := range email.Attachments {
		attachments = append(attachments, attachment)
	}

	// Prefer HTML if available
	if email.HTMLBody != "" {
		mailBody, html2mdErr := html2markdown.NewConverter("", true, nil).ConvertString(email.HTMLBody)
		if html2mdErr == nil {
			return email.Subject, mailBody, date, fromNames, email.MessageID, attachments, nil
		}
		p.API.LogError("Error in converting html to markdown", "err", html2mdErr.Error())
	}

	return email.Subject, email.TextBody, date, fromNames, email.MessageID, attachments, nil
}

func (p *Plugin) getAttachmentDetails(attachment parsemail.Attachment) (string, []byte) {
	bytesData, err := ioutil.ReadAll(attachment.Data)
	if err != nil {
		p.API.LogError("Error occured in reading attachments", "err", err.Error())
		return "", []byte{}
	}
	return attachment.Filename, bytesData
}

func (p *Plugin) handleMessages(messages []*gmail.Message, channelID string, userID string, notify bool) error {
	if len(messages) == 0 {
		return errors.New("No message found")
	}

	postAsID := userID
	if notify {
		postAsID = p.gmailBotID
	}

	parentID := ""
	rootID := ""
	for messageIndex, message := range messages {
		base64URLMessage := message.Raw
		plainTextMessage, err := p.decodeBase64URL(base64URLMessage)
		if err != nil {
			p.API.LogError("Error occured in decoding base64 URL message", "err", err.Error())
			return err
		}

		// Extract Subject and Body (base64url) from the message.
		subject, body, date, from, rfcID, attachments, err := p.parseMessage(plainTextMessage)
		sharingInfo := ""
		if notify {
			sharingInfo = "**Message ID: <" + rfcID + ">**. _(Import in any channel using `/gmail import <mail/thread> <ID>`)_\n\n"
		}
		if err != nil {
			p.API.LogError("An error has occured while trying to parse the mail", "err", err.Error())
			return err
		}
		if from == "" {
			from = "_Could not fetch names_"
		}

		fileIDArray := []string{}
		fileNameArray := []string{}
		for _, attachment := range attachments {
			fileName, fileData := p.getAttachmentDetails(attachment)
			fileInfo, fileErr := p.API.UploadFile(fileData, channelID, fileName)
			if fileErr != nil {
				p.API.LogError("Attachment "+fileName+" could not be uploaded", "err", err.Error())
			}
			fileNameArray = append(fileNameArray, fileName)
			fileIDArray = append(fileIDArray, fileInfo.Id)
		}
		// Prepare post for posting as a response

		if messageIndex == 0 {
			rootPost := &model.Post{
				UserId:    postAsID,
				ChannelId: channelID,
				Message:   "###### Email from : " + from + "\n\n" + sharingInfo + "**Date: " + date + "** \n\n" + "**Subject: " + subject + "**\n\n" + body,
			}
			rootPost, _ = p.API.CreatePost(rootPost)
			rootID = rootPost.Id
			parentID = rootID
		} else {
			// Can assume that rootID is not ""
			post := &model.Post{
				UserId:    postAsID,
				ChannelId: channelID,
				RootId:    rootID,
				ParentId:  parentID,
				Message:   "###### Email from: " + from + "\n\n" + sharingInfo + "**Date: " + date + "** \n\n" + "**Subject: " + subject + "**\n\n" + body,
			}
			postInfo, _ := p.API.CreatePost(post)
			parentID = postInfo.Id
		}

		// Post attachments
		if len(fileIDArray) > 0 {
			countFiles := 0
			// One Post can contain atmost 5 attachments
			for countFiles = 0; countFiles <= len(fileIDArray); countFiles += 5 {
				post := &model.Post{
					UserId:    postAsID,
					ChannelId: channelID,
					RootId:    rootID,
					ParentId:  parentID,
					FileIds:   fileIDArray[countFiles:int(math.Min(float64(countFiles+5), float64(len(fileIDArray))))],
				}
				postInfo, err := p.API.CreatePost(post)
				parentID = postInfo.Id
				if err != nil {
					p.API.LogError("Could not create post", "err", err.Error())
					return err
				}
			}
		}
	}
	return nil
}

// subscribeToLabels
func (p *Plugin) subscribeToLabels(userID string, gmailID string, labelIDs []string) error {
	gmailService, _ := p.getGmailService(userID)
	watchRequest := &gmail.WatchRequest{
		LabelFilterAction: "include",
		LabelIds:          labelIDs,
		TopicName:         p.getConfiguration().TopicName,
	}
	watchResponse, err := gmailService.Users.Watch(gmailID, watchRequest).Do()
	if err != nil {
		p.API.LogError("Could not subscribe user to the supported labels", "err", err.Error())
		return err
	}
	p.updateHistoryIDForUser(uint64(watchResponse.HistoryId), userID)
	p.updateSubscriptionsOfUser(userID, labelIDs)
	return nil
}

// getRelevantMessagesForUser filters messages that have a label the user is subscribed to
func (p *Plugin) getRelevantMessagesForUser(userID string, messages []*gmail.Message) []*gmail.Message {
	subscriptions, _ := p.getSubscriptionsOfUser(userID)
	relevantMessages := []*gmail.Message{}
	// TODO: OPTIMIZATION
	messageAdded := false
	for _, message := range messages {
		messageAdded = false
		for _, subscription := range subscriptions {
			for _, messageLabel := range message.LabelIds {
				if messageLabel == subscription {
					relevantMessages = append(relevantMessages, message)
					messageAdded = true
					break
				}
			}
			if messageAdded {
				break
			}
		}
	}
	return relevantMessages
}

// getSupportedLabels
func (p *Plugin) getSupportedLabels() []string {
	labels := []string{}
	for label := range supportedLabelIDs {
		labels = append(labels, label)
	}
	return labels
}
