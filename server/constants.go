package main

// commands in plugin
const (
	commandGmail = "gmail"
)

// specific to plugin sub-commands
const (
	helpText = "##### Mattermost Gmail Plugin\nAvailable Commands:\n" +
	` /gmail connect` + "- Connect your Gmail (Google mail account) with your Mattermost account\n**NOTE:** To disconnect Gmail from your Mattermost account, head over to Gmail, click on your profile picture > Select 'Manage your Google Account' > Select 'Security' > Expand 'Third Party Access' > Remove access\n" +
	 `/gmail import mail [id]` + "- Imports a mail (message) from Gmail using ID of the mail (message)\n**Tip: ** To get ID of the mail (message), click on the 3 dots in the mail, then select 'Show Original' to obtain the Message ID.\n" +
	 `/gmail import thread [id]` + "- Import a conversation (thread) from Gmail using ID of any message in the conversation (thread)\n"
	 `/gmail help`  + "- Display this help"
)

// specific to scope required
const (
	emailScope = "https://www.googleapis.com/auth/userinfo.email"
)
