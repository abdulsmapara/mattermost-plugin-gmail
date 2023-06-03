package main

// commands in plugin
const (
	commandGmail = "gmail"
)

// specific to plugin sub-commands
const (
	helpTextHeader = "###### Mattermost Gmail Plugin - Slash Command Help\n"

	commonHelpText = "\n* `/gmail connect` - Connect your Mattermost account to your Gmail account\n" +
		"* `/gmail disconnect` - Disconnect Gmail from Mattermost\n" +
		"* `/gmail import mail <message-id>` - Import a mail/message from Gmail using message ID.\n\nNote: To get ID of any mail, click on the 3 dots after opening the mail, and then select 'Show Original'. You will see the Message ID at the top in a new tab\n" +
		"* `/gmail import thread <thread-message-id>` - Import a complete Gmail thread (conversation) using ID of any mail in the thread\n" +
		"* `/gmail subscribe <optional-label-ids>` - Subscribe to get notifications from the Gmail Bot for the labels mentioned. Mention the label IDs in comma-separated fashion from the list: INBOX, CATEGORY_PERSONAL, CATEGORY_SOCIAL, CATEGORY_PROMOTIONS, CATEGORY_UPDATES, CATEGORY_FORUMS. The default label is INBOX.\n" +
		"* `/gmail unsubscribe <optional-label-ids>` - Unsubscribe from the mentioned labels (should be comma-separated). If none is mentioned, you'll be unsubscribed from all the label IDs. It might take a few minutes for the effect to take place.\n" +
		"* `/gmail subscriptions` - Display label IDs currently subscribed to\n" +
		"* `/gmail help` - Display help about this plugin"
)

const (
	// ActionDisconnectPlugin is used in Post action to identify disconnect button action
	ActionDisconnectPlugin = "ActionDisconnectPlugin"
	// ActionCancel can be used in any Post action to identify cancel action
	ActionCancel = "ActionCancel"
)

// specific to scope required
const (
	emailScope = "https://www.googleapis.com/auth/userinfo.email"
)

// label IDs supported
// Note: supportedLabelIDs used as set data structure
var supportedLabelIDs = map[string]int{
	/* Label */            /* Any valid int*/
	"INBOX":               1,
	"CATEGORY_PROMOTIONS": 2,
	"CATEGORY_PERSONAL":   3,
	"CATEGORY_SOCIAL":     4,
	"CATEGORY_UPDATES":    5,
	"CATEGORY_FORUMS":     6,
}
