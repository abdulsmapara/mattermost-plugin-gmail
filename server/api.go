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

	// Create a unique ID generated to protect against CSRF attach while auth.
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

	htmlMessage := `
	<!DOCTYPE html>
	<html>
		<head>
			<script>
				window.close();
			</script>
		</head>
		<body>
			<p>Completed connecting to Gmail successfully. Please close this window and head back to the Mattermost application.</p>
		</body>
	</html>
	`

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

	p.mailNotificationDetails.UserID = userID

	// Extract the access code from the redirected url
	accessCode := r.URL.Query().Get("code")

	// Create a context
	ctx := context.Background()

	oauthConf := p.getOAuthConfig()

	// Exchange the access code for access token from Google (gmail) token url
	token, appErr := oauthConf.Exchange(ctx, accessCode)
	if appErr != nil {
		http.Error(w, appErr.Error(), http.StatusInternalServerError)
		return
	}

	tokenJSON, jsonErr := json.Marshal(token)
	if jsonErr != nil {
		http.Error(w, "Invalid token marshal", http.StatusBadRequest)
		return
	}

	p.API.KVSet(userID+"gmailToken", tokenJSON)
	// Post intro post
	message := "#### Welcome to the Mattermost Gmail Plugin!\n" +
		"You've successfully connected your Mattermost account to your Gmail.\n" +
		"Please type `/gmail help` to understand how to use this plugin."

	p.CreateBotDMPost(userID, message)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprint(w, htmlMessage)

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
	actionToBeTaken := intergrationResponseFromCommand.Context["action"].(string)
	channelID := intergrationResponseFromCommand.ChannelId
	originalPostID := intergrationResponseFromCommand.PostId
	actionSecret := p.getConfiguration().EncryptionKey
	actionSecretPassed := intergrationResponseFromCommand.Context["actionSecret"].(string)

	if actionToBeTaken == ActionDisconnectPlugin && actionSecret == actionSecretPassed {
		// Unique identifier
		accessTokenIdentifier := userID + "gmailToken"

		// Delete the access token from KV store
		err := p.API.KVDelete(accessTokenIdentifier)
		if err != nil {
			p.API.DeleteEphemeralPost(userID, originalPostID)
			p.sendMessageFromBot(channelID, userID, true, fmt.Sprintf("Unable to disconnect Gmail: %v", err.Error()))
			return
		}

		// Send and override success disconnect message
		p.API.UpdateEphemeralPost(userID, &model.Post{
			Id:        originalPostID,
			UserId:    p.gmailBotID,
			ChannelId: channelID,
			Message: fmt.Sprint(
				":zzz: You have successfully disconnect your Gmail with Mattermost. You may also perform this steps: Gmail Profile Picture Icon > Manage Your Google Account > Security Issues > Third Party Access > Remove Access by this project.\n" +
					"If you ever want to connect again, just use `/gmail connect`"),
		})
		return
	}

	if actionToBeTaken == ActionCancel && actionSecret == actionSecretPassed {
		p.API.UpdateEphemeralPost(userID, &model.Post{
			Id:        originalPostID,
			UserId:    p.gmailBotID,
			ChannelId: channelID,
			Message:   fmt.Sprint(channelID + " "),
		})
		return
	}

	// If secret don't match or action is not the one we want.
	http.Error(w, "Unauthorized or unknown disconnect action detected", http.StatusInternalServerError)
	p.API.DeleteEphemeralPost(userID, originalPostID)
}

func (p *Plugin) sendMailNotification(w http.ResponseWriter, r *http.Request) {
	p.API.LogInfo("Received Gmail Notification")

	fbody := r.Body

	buf := new(bytes.Buffer)
	buf.ReadFrom(fbody)
	body := buf.String()

	var parsedBody map[string]interface{}
	json.Unmarshal([]byte(body), &parsedBody)

	requestMessage := parsedBody["message"].(map[string]interface{})
	data := requestMessage["data"].(string)
	decodedData, _ := p.decodeBase64URL(data)

	var parsedData map[string]interface{}
	json.Unmarshal([]byte(decodedData), &parsedData)
	emailAddress := parsedData["emailAddress"].(string)
	fmt.Println(emailAddress)
	historyID := uint64(parsedData["historyId"].(float64))

	userID := p.mailNotificationDetails.UserID

	gmailService, srvErr := p.getGmailService(userID)
	if srvErr != nil {
		p.API.LogInfo("Could not get gmail service: " + srvErr.Error())
		w.WriteHeader(200)
		return
	}

	historyResponse, histErr := gmailService.Users.History.List(emailAddress).StartHistoryId(p.mailNotificationDetails.HistoryID).Do()

	if histErr != nil {
		p.API.LogInfo("Could not fetch history details: " + histErr.Error())
		w.WriteHeader(200)
		return
	}
	if len(historyResponse.History) < 1 {
		p.API.LogInfo("Unable to fetch history")
		w.WriteHeader(200)
		return
	}
	// Update historyID for next time
	p.mailNotificationDetails.HistoryID = historyID

	lastHistoryIndex := len(historyResponse.History) - 1
	addedMessages := historyResponse.History[lastHistoryIndex].MessagesAdded

	messages := []*gmail.Message{}
	for _, addedMessage := range addedMessages {
		message, _ := gmailService.Users.Messages.Get(emailAddress, addedMessage.Message.Id).Format("raw").Do()
		fmt.Println(message)
		messages = append(messages, message)
	}
	directChannel, err := p.API.GetDirectChannel(userID, p.gmailBotID)
	if err != nil {
		p.API.LogInfo("Could not fetch direct channel")
		w.WriteHeader(200)
		return
	}

	erro := p.handleMessages(messages, directChannel.Id, userID, true)
	if erro != nil {
		p.API.LogInfo("Message not posted: " + erro.Error())
	}

	w.WriteHeader(200)
}
