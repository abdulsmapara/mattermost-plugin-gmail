package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
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
	http.Redirect(w, r, oAuthconfig.AuthCodeURL(antiCSRFToken), http.StatusFound)
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
	fmt.Println(token)
	// Post intro post
	message := "#### Welcome to the Mattermost Gmail Plugin!\n" +
		"You've successfully connected your Mattermost account to your Gmail.\n" +
		"Please type `/gmail help` to understand how to use this plugin."

	p.CreateBotDMPost(userID, message)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, htmlMessage)

}
