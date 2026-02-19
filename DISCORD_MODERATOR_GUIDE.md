# Discord Moderator Guide for Athena

This guide explains how Discord moderators can use the webhook integration to help moderate the Athena server.

## Overview

Athena can send moderation notifications directly to a Discord channel using webhooks. This allows Discord moderators to monitor server activity and respond to moderation requests without being connected to the AO server itself.

## Setup

### Creating a Discord Webhook

1. Open your Discord server and navigate to the channel where you want to receive moderation notifications
2. Click the gear icon next to the channel name to open Channel Settings
3. Navigate to **Integrations** → **Webhooks**
4. Click **New Webhook** or **Create Webhook**
5. Give your webhook a name (e.g., "Athena Moderation")
6. Copy the webhook URL

### Configuring Athena

1. Open your `config/config.toml` file
2. Find the `webhook_url` setting under the `[Server]` section
3. Paste your Discord webhook URL:
   ```toml
   webhook_url = "https://discord.com/api/webhooks/YOUR_WEBHOOK_URL_HERE"
   ```
4. Restart your Athena server

## Features

### Modcall Notifications

When a player calls for a moderator using `/modcall` in-game, a notification is automatically sent to your Discord channel containing:

- **Character name** - Who called for help
- **Area** - Which area they're in
- **Reason** - Why they need a moderator

Example notification:
```
Phoenix Wright sent a modcall in Courtroom, No. 1

Reason: User is spamming inappropriate messages
```

### Report Files

When a modcall is made, the server automatically generates a report file containing:
- Recent chat history from that area
- Player actions and commands
- Timestamps for all events

This report is uploaded to Discord so moderators can review the full context.

## Best Practices for Discord Moderators

### Response Guidelines

1. **Acknowledge quickly** - React to modcall notifications to show you've seen them
2. **Review the report** - Read the uploaded report file for full context
3. **Coordinate with in-game moderators** - Use Discord to discuss before taking action
4. **Document decisions** - Keep notes in Discord about moderation actions taken

### Security Considerations

- **Keep webhook URLs private** - Anyone with the URL can post to your channel
- **Limit channel access** - Only give moderators access to the moderation channel
- **Rotate webhooks periodically** - If a webhook URL is compromised, delete it and create a new one
- **Don't share player information** - Report files may contain private player data

## Performance and Resource Usage

The webhook integration is designed to be efficient and non-blocking:

- **Asynchronous posting** - Webhook messages are queued and sent in the background
- **No server delays** - In-game modcalls are not delayed waiting for Discord
- **Automatic retry** - Failed messages are logged but don't crash the server
- **Memory efficient** - Queue has a fixed size to prevent memory leaks

The webhook queue can hold up to 100 pending notifications. If the queue fills up (which would only happen if Discord is unreachable for an extended period), new notifications will be dropped rather than blocking the game server.

## Troubleshooting

### Notifications Not Appearing

1. **Check webhook URL** - Ensure it's correctly set in `config.toml`
2. **Verify webhook is active** - Go to Discord channel settings to confirm it exists
3. **Check server logs** - Look for webhook-related error messages
4. **Test the webhook** - Use a curl command or Discord's "Send Test Message" feature

### Delayed Notifications

- **Network issues** - Check your server's internet connection
- **Discord API limits** - Discord rate limits webhook posts (typically 30/minute per webhook)
- **Queue backup** - If many modcalls happen simultaneously, they'll be processed in order

### Report Files Not Uploading

- **File size limits** - Discord has a file size limit (typically 8MB for most servers)
- **Permissions** - Ensure the webhook has permission to upload files
- **Log buffer size** - Check the `log_buffer_size` setting in your config

## Advanced Configuration

### Custom Server Name

The webhook posts use your server name from `config.toml`:

```toml
name = "My Awesome Server"
```

This appears as the webhook username in Discord notifications.

### Log Buffer Size

Control how much history is included in report files:

```toml
[Logging]
log_buffer_size = 150  # Number of events to include in reports
```

Larger buffers provide more context but create larger files.

## Getting Help

If you encounter issues with the webhook integration:

1. Check the Athena server logs for error messages
2. Verify your Discord webhook is still active
3. Test connectivity between your server and Discord's API
4. Consult the main [Athena documentation](https://github.com/MangosArentLiterature/Athena)

## Privacy Note

Remember that modcall reports contain player chat history and may include sensitive information. Treat all moderation data with appropriate confidentiality and follow your server's privacy policies.
