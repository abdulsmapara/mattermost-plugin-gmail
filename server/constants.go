package main

// commands in plugin
const (
	commandGmail = "gmail"
)

// specific to plugin sub-commands
const (
	helpTextHeader = "###### Mattermost Gmail Plugin - Slash Command Help\n"

	commonHelpText = "\n* `/gmail connect` - Connect your Mattermost account to your Gmail account\n" +
		"* `/gmail import mail <message-id>` - Import a mail/message from Gmail using message ID.\n" +
		"* `/gmail import thread <thread-message-id>` - Import complete Gmail thread (conversation) using ID of any mail in the thread\n" +
		"* `/gmail help` - Display help about this plugin\n" +
		"* Tips:\n" +
		"	- To disconnect (command coming soon), head over to your Gmail, click on the profile picture icon, select 'Manage Your Google Account', select 'Security Issues', then select 'Third party access', and finally remove Mattermost access\n" +
		"	- To get ID of any mail, click on the 3 dots after opening the mail, and then select 'Show Original'. You will see 'Message ID' at the top in a new tab"
)

// specific to scope required
const (
	emailScope = "https://www.googleapis.com/auth/userinfo.email"
)
