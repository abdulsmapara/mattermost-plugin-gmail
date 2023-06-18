package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"net/http"
	"strconv"
	"strings"
)

// ServeHTTP allows the plugin to implement the http.Handler interface. Requests destined for the
// /plugins/{id} path will be routed to the plugin.
//
// The Mattermost-User-Id header will be present if (and only if) the request is by an
// authenticated user.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch path := r.URL.Path; path {
	case "/oauth/connect":
		p.connectGmail(w, r)
	case "/oauth/complete":
		p.completeGmailConnection(w, r)
	case "/command/disconnect":
		p.disconnectGmail(w, r)
	case "/webhook/gmail":
		p.sendMailNotification(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (p *Plugin) connectGmail(w http.ResponseWriter, r *http.Request) {
	authedUserID := r.Header.Get("Mattermost-User-ID")

	// Unauthorized user
	if authedUserID == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	// Create a unique ID generated to protect against CSRF attack while auth.
	antiCSRFToken := fmt.Sprintf("%v_%v", model.NewId()[0:15], authedUserID)

	// Store that uniqueState for later validations in redirect from oauth
	if err := p.API.KVSet(antiCSRFToken, []byte(antiCSRFToken)); err != nil {
		http.Error(w, "Failed to save state", http.StatusBadRequest)
		return
	}

	// Get OAuth configuration
	oAuthconfig := p.getOAuthConfig()

	// Redirect user to auth URL for authentication
	http.Redirect(w, r, oAuthconfig.AuthCodeURL(antiCSRFToken, oauth2.AccessTypeOffline, oauth2.ApprovalForce), http.StatusTemporaryRedirect)
}

func (p *Plugin) completeGmailConnection(w http.ResponseWriter, r *http.Request) {

	// Check if we were redirected from Mattermost pages
	authUserID := r.Header.Get("Mattermost-User-ID")
	if authUserID == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	// Get the state "antiCSRFToken" we passed in earlier when redirecting to Google (gmail) auth URL from redirect URL
	antiCSRFTokenInURL := r.URL.Query().Get("state")

	// Check if antiCSRFToken is the same in redirect URL as to which we passed in earlier
	antiCSRFTokenPassedEarlier, err := p.API.KVGet(antiCSRFTokenInURL)
	if err != nil {
		http.Error(w, "AntiCSRF state not found", http.StatusBadRequest)
		return
	}

	if string(antiCSRFTokenPassedEarlier) != antiCSRFTokenInURL || len(antiCSRFTokenInURL) == 0 {
		http.Error(w, "Cross-site request forgery", http.StatusForbidden)
		return
	}

	// Extract user id from the state
	userID := strings.Split(antiCSRFTokenInURL, "_")[1]

	// and then clear the KVStore off the CSRF token
	p.API.KVDelete(antiCSRFTokenInURL)

	// Check if the same user in MM who is authenticated with Google (gmail)
	if userID != authUserID {
		http.Error(w, "Incorrect user while authentication", http.StatusUnauthorized)
		return
	}

	// Extract the access code from the redirected url
	accessCode := r.URL.Query().Get("code")

	// Create a context
	ctx := context.Background()

	oauthConf := p.getOAuthConfig()

	// Exchange the access code for access token from Google (gmail) token url
	token, appErr := oauthConf.Exchange(ctx, accessCode)
	if appErr != nil {
		p.API.LogError("Error while exchanging access code for access token from Gmail token url", "err", appErr.Error())
		http.Error(w, appErr.Error(), http.StatusInternalServerError)
		return
	}

	tokenJSON, jsonErr := json.Marshal(token)
	if jsonErr != nil {
		http.Error(w, "Invalid token marshal", http.StatusBadRequest)
		return
	}

	p.API.LogInfo("Starting to onboard user with user ID: " + userID)
	onBoardErr := p.onboardUser(userID, tokenJSON)
	if onBoardErr != nil {
		p.API.LogError("Error occured - Could not onboard user", "err", onBoardErr.Error())
		p.CreateBotDMPost(userID, "Error occured while connecting to Gmail. Please try again later.")
		return
	}
	p.API.LogDebug("Onboarding completed successfully for user with user ID: " + userID)

	// Post intro post
	message := "#### Welcome to the Mattermost Gmail Plugin!\n" +
		"You've successfully connected your Mattermost account to your Gmail.\n" +
		"Please type `/gmail help` to understand how to use this plugin."

	p.CreateBotDMPost(userID, message)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprint(w, htmlMessageOnCompletingGmailConnection)

}

func (p *Plugin) disconnectGmail(w http.ResponseWriter, r *http.Request) {
	// Check if this was passed within Mattermost
	authUserID := r.Header.Get("Mattermost-User-ID")
	if authUserID == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	// Get the information from Body which contain the interactive Message Attachment we sent from /disconnect command
	intergrationResponseFromCommand := model.PostActionIntegrationRequestFromJson(r.Body)

	userID := intergrationResponseFromCommand.UserId
	actionToBeTaken, _ := intergrationResponseFromCommand.Context["action"].(string)
	channelID := intergrationResponseFromCommand.ChannelId
	originalPostID := intergrationResponseFromCommand.PostId

	if actionToBeTaken == ActionDisconnectPlugin {
		err := p.offboardUser(userID)

		if err != nil {
			p.API.DeleteEphemeralPost(userID, originalPostID)
			p.sendMessageFromBot(channelID, userID, true, "Unable to disconnect Gmail. Please try again later.")
			errorMessage := err.Error()
			p.API.LogError("Error occured while disconnecting user from Gmail. Offboarding failed.", "err", errorMessage)
			http.Error(w, "Error occured while disconnecting user from Gmail.", http.StatusInternalServerError)
			return
		}

		// Send and override success disconnect message
		p.API.UpdateEphemeralPost(userID, &model.Post{
			Id:        originalPostID,
			UserId:    p.gmailBotID,
			ChannelId: channelID,
			Message: fmt.Sprint(
				":zzz: You have successfully disconnected your Gmail with Mattermost. You may also perform these steps: Gmail Profile Picture Icon > Manage Your Google Account > Security Issues > Third Party Access > Remove Access by this project.\n" +
					"If you ever want to connect again, just use `/gmail connect`"),
		})
		return
	}

	if actionToBeTaken == ActionCancel {
		p.API.UpdateEphemeralPost(userID, &model.Post{
			Id:        originalPostID,
			UserId:    p.gmailBotID,
			ChannelId: channelID,
			Message:   fmt.Sprint("Disconnect attempt cancelled successfully."),
		})
		return
	}

	// If secret don't match or action is not the one we want.
	http.Error(w, "Unauthorized or unknown disconnect action detected", http.StatusInternalServerError)
	p.API.DeleteEphemeralPost(userID, originalPostID)
}

func (p *Plugin) sendMailNotification(w http.ResponseWriter, r *http.Request) {
	// If the body isn't of type json, then reject
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "Content types don't match", http.StatusBadRequest)
		return
	}

	p.API.LogDebug("Received Gmail Notification")
	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)
	body := buf.String()

	var parsedBody map[string]interface{}
	err := json.Unmarshal([]byte(body), &parsedBody)
	if err != nil {
		http.Error(w, "Cannot unmarshal input json", http.StatusBadRequest)
		return
	}

	requestMessage := parsedBody["message"].(map[string]interface{})
	data := requestMessage["data"].(string)
	decodedData, _ := p.decodeBase64URL(data)

	var parsedData map[string]interface{}
	json.Unmarshal([]byte(decodedData), &parsedData)
	emailAddress := parsedData["emailAddress"].(string)
	historyID := uint64(parsedData["historyId"].(float64))
	userIDs, _ := p.getUsersForGmail(emailAddress)

	p.API.LogInfo("Received Gmail notification for users connected to gmail ID: " + emailAddress)

	if len(userIDs) < 1 {
		p.API.LogInfo("No user connected to gmail ID: " + emailAddress)
		w.WriteHeader(200)
		return
	}

	p.API.LogInfo(fmt.Sprintf("%d users connected to gmail ID: %s", len(userIDs), emailAddress))

	for _, userID := range userIDs {
		p.API.LogInfo("Processing notification for userID: " + userID)

		gmailService, srvErr := p.getGmailService(userID)
		if srvErr != nil {
			p.API.LogError("Could not get gmail service for user with user ID: "+userID, "err", srvErr.Error())
			continue
		}

		lastHistoryID, err := p.getHistoryIDForUser(userID)
		if err != nil {
			p.API.LogError("Could not fetch history details for user with user ID: "+userID, "err", err.Error())
			continue
		}

		p.API.LogInfo("Fetching gmail messages using last used history ID: " + strconv.Itoa(int(lastHistoryID)))
		historyResponse, histErr := gmailService.Users.History.List(emailAddress).StartHistoryId(lastHistoryID).Do()
		if histErr != nil {
			p.API.LogError("Could not fetch history response for user with user ID: "+userID, "err", histErr.Error())
			continue
		}
		if len(historyResponse.History) < 1 {
			p.API.LogInfo("Blank history response received for user with user ID: " + userID)
			continue
		}

		lastHistoryIndex := len(historyResponse.History) - 1
		addedMessages := historyResponse.History[lastHistoryIndex].MessagesAdded
		messages := []*gmail.Message{}

		for _, addedMessage := range addedMessages {
			message, _ := gmailService.Users.Messages.Get(emailAddress, addedMessage.Message.Id).Format("raw").Do()
			messages = append(messages, message)
		}
		p.API.LogInfo(fmt.Sprintf("%d messages received as a part of the notification, filtering based on user's subscriptions", len(messages)))
		relevantMessages := p.getRelevantMessagesForUser(userID, messages)
		if len(relevantMessages) < 1 {
			p.API.LogInfo("No new relevant messages found for the user")
			continue
		}
		p.API.LogInfo(fmt.Sprintf("%d messages relevant based on user's subscriptions", len(relevantMessages)))
		directChannel, channelErr := p.API.GetDirectChannel(userID, p.gmailBotID)
		if channelErr != nil {
			p.API.LogError("Could not fetch direct channel for the user", "err", channelErr.Error())
			continue
		}
		msgErr := p.handleMessages(relevantMessages, directChannel.Id, userID, true)
		if msgErr != nil {
			p.API.LogError("Message could not be posted to the user", "err", msgErr.Error())
			continue
		}
		p.API.LogInfo("Updating history ID for the user")
		updateErr := p.updateHistoryIDForUser(historyID, userID)
		if updateErr != nil {
			p.API.LogError("Could not update history ID for the user", "err", updateErr.Error())
			continue
		}
		p.API.LogInfo("History ID updated for the user")

	}
	p.API.LogInfo(fmt.Sprintf("Processed notifications for %d users", len(userIDs)))
	w.WriteHeader(200)
	return
}
