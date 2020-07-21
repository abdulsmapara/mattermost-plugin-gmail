package main

// commands in plugin
const (
	commandGmail = "gmail"
)

// specific to plugin sub-commands
const (
	helpText = "##### Mattermost Gmail Plugin\nAvailable Commands:\n" +
		`* |/gmail connect| - Connect your Gmail (Google mail account) with your Mattermost account
	 * |/gmail import mail [id]| - Imports a mail (message) from Gmail using its ID
	 * |/gmail import thread [id]| - Import a conversation (thread) from Gmail using its ID
	 * |/gmail help| - Display this help
	`
)

// specific to scope required
const (
	emailScope = "https://www.googleapis.com/auth/userinfo.email"
)
