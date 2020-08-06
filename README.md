# Mattermost Gmail Plugin
**A Gmail plugin for [Mattermost](https://mattermost.com) - Brings your gmail conversations within Mattermost**

**Developer:** [@abdulsmapara](https://github.com/abdulsmapara)

## Table of content
- [About the plugin](#about-the-plugin)
- [Installation](#installation)
- [Connecting with Gmail](#connecting-with-gmail)

## About the plugin
The plugin connects your Gmail with Mattermost, that enables you to import Gmail messages and threads (along with attachments) to any Mattermost channel. Also, you can subscribe to get notifications on new emails. Explore the plugin and report issues, if any, to [Support-Page](https://github.com/abdulsmapara/mattermost-plugin-gmail/issues).

## Installation
1. Download the latest version of the [release](https://github.com/abdulsmapara/mattermost-plugin-gmail/releases) directory. Go to `System Console` and upload the latest release in the Plugin Management section. For help on how to install a custom plugin, please refer [installing custom plugin docs](https://docs.mattermost.com/administration/plugins.html#custom-plugins).

1. Next, you will need to enter Client Secret, Client ID, Topic Name and generate Encryption Key to enable the plugin successfully. You will also need to create a Google Pub/Sub Subscription for the plugin to function properly -

	1. To obtain Client Secret & Client ID -
		* Go to [Google Cloud Dashboard](https://console.cloud.google.com/home/dashboard) and create a new project.
		* After creating a project click on `Go to APIs overview` on `APIs` card from the dashboard which will take you to the API dashboard.
		* From the left menu, select `OAuth consent screen` (This page configures the consent screen for all applications in the created project)
		* Select `Internal` (In this mode, the app is limited to G Suite users within a organization) or `External` (In this mode, the app is available to any user with a Google account)
		* Type the Application Name (eg. Mattermost Gmail Plugin), upload the Application logo, select a support mail.
		* Next, Click on `Add Scope` and add the scope `https://mail.google.com/`. If you cannot view this scope, click on `enabled APIs` link in the header (opens in new tab), select `Enable APIs and Services`, search for (and select) `Gmail API` and then click `Enable API`. Now, you should see the required scope.
		* Click on `Save` and select `Credentials` from the left menu
		* Click on `Create Credentials` and select `OAuth client ID` from the dropdown
		* Select the Application type as `Web Application`
		* Enter the name of the OAuth 2.0 client (not shown to end users)
		* Enter the values of `Authorized Javascript Origins` as `<Mattermost server URL>` and the value of `Authorised redirect URIs` as `<Mattermost server URL>/plugins/mattermost-plugin-gmail/oauth/complete` and then click on `Create`.
		* Copy the Client ID and Client Secret and enter these in the Plugin Configuration Settings.

	2. To obtain the topic name -
		* Open the navigation menu (by clicking on Hamburger icon), scroll down and find `Pub/Sub` in the `Big Data` section. Select `Topics` from the menu obtained on hovering over the `Pub/Sub` title.
		* Create a Topic by entering a `Topic ID` (eg. mattermost-gmail-plugin-topic) and selecting a suitable option for `Encryption` (If you select, `Customer-managed key`, you may need to configure a little to proceed).
		* Select `Add Member` from the Permissions Section present on the right.
		* Enter `gmail-api-push@system.gserviceaccount.com` in the `New members` field.
		* Select the role as `Pub/Sub Publisher` from `Pub/Sub` in the dropdown and click on `Save`.
		* You can now copy the displayed `Topic Name` and enter in the Plugin Configuration Settings (eg. `projects/mattermost-project-111111/topics/mattermost-gmail-plugin-topic`).

	3. Create a subscription -
		* Select `Subscriptions` from the left menu and click on `Create Subscription`
		* Provide a `Subscription ID` (eg. `mattermost-plugin-gmail-subscription`)
		* Select the Topic just created by following the above steps
		* Select the `Delivery Type` as `Push`
		* Enter the `Endpoint URL` as `<Mattermost-Server-URL>/plugins/mattermost-plugin-gmail/webhook/gmail` (_Currently, do not check `Enable Authentication`._)
		* Choose `Never Expire` for `Subscription expiration`
		* Set `Acknowledgement deadline` to anything between `10 seconds` to `600 seconds`
		* You can let other fields being set to default values or configure them if you wish
		* Click on `Create` to complete creation of subscription.

	4. Generate the Encryption Key -
		* In the Plugin Configuration Settings, if Encryption Key is empty, simply click on `Regenerate` button just below the field for it.

1. You are now set to use the Plugin.

## Connecting with Gmail

1. Head over to any Mattermost channel, and type the slash command - `/gmail connect`. The Gmail Bot will post a link, through which you can connect your Gmail Account.

2. Click on the link, and select the Gmail Account that you wish to connect.

3. You then need to grant certain permissions to proceed.

4. Once you grant the permissions, you will be redirected to a Successfully authenticated page, which you can close and head back to the Mattermost Application.

5. A new direct message from the Gmail Bot is also posted stating the same. With this your Gmail account is successfully connected to Mattermost.

