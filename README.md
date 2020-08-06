# Mattermost Gmail Plugin
**A Gmail plugin for [Mattermost](https://mattermost.com) - Brings your gmail conversations within Mattermost**

**Developer:** [@abdulsmapara](https://github.com/abdulsmapara)

## Table of content
- [About the plugin](#about-the-plugin)
- [Installation](#installation)
- [Features](#features)

## About the plugin
The plugin connects your Gmail with Mattermost, that enables you to import Gmail messages and threads (along with attachments) to any Mattermost channel. Also, you can subscribe to get notifications on new emails. Explore the plugin and report issues, if any, to [Support-Page](https://github.com/abdulsmapara/mattermost-plugin-gmail/issues).

## Installation
1. Download the latest version of the [release](https://github.com/abdulsmapara/mattermost-plugin-gmail/releases) directory. Go to `System Console` and upload the latest release in the Plugin Management section. For help on how to install a custom plugin, please refer [installing custom plugin docs](https://docs.mattermost.com/administration/plugins.html#custom-plugins).

1. Next, you will need to enter Client Secret, Client ID, Topic Name and generate Encryption Key to enable the plugin successfully -

	1. To obtain Client Secret & Client ID -
		* Go to [Google Cloud Dashboard](https://console.cloud.google.com/home/dashboard) and create a new project.