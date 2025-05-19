# MuChat Bot Plugin for Mattermost

MuChat Bot is a Mattermost plugin that integrates conversational AI capabilities into your Mattermost workspace. It allows users to interact with a MuChat agent directly from Mattermost channels using mentions or slash commands.

## Features

- Responds to mentions with `@muchat` in public channels.
- Automatically handles direct messages (DMs).
- Configurable API key and agent ID for MuChat integration.
- Optional debug mode for enhanced logging.

## Requirements

- Mattermost Server version 9.11.0 or higher.
- Node.js v16 and npm v8 for development.

## Installation

1. Build the plugin:

   ```bash
   make clean && make dist
   ```

2. Upload the plugin to your Mattermost server:

   ```bash
   docker cp dist/com.pardis.muchat-0.1.0.tar.gz mattermost-dev:/tmp/
   docker exec -it mattermost-dev mmctl --local plugin add /tmp/com.pardis.muchat-0.1.0.tar.gz
   docker exec -it mattermost-dev mmctl --local plugin enable com.pardis.muchat
   ```

## Configuration

1. Navigate to **System Console > Plugins > MuChat Bot**.
2. Configure the following settings:

   - **MuChat API Key**: The API key for authenticating with the MuChat service.
   - **Agent ID**: The ID of the MuChat agent to forward messages to.
   - **Enable Debug Mode**: Enable or disable debug logging.

## Usage

- Mention `@muchat` in a public channel to interact with the bot.
- Send a direct message to the bot for private interactions.

## Development

### Prerequisites

- Install dependencies:

  ```bash
  nvm install
  npm install
  ```

### Build and Deploy

- Build the plugin:

  ```bash
  make clean && make dist
  ```

- Deploy the plugin locally:

  ```bash
  docker cp dist/com.pardis.muchat-0.1.0.tar.gz mattermost-dev:/tmp/
  docker exec -it mattermost-dev mmctl --local plugin add /tmp/com.pardis.muchat-0.1.0.tar.gz
  docker exec -it mattermost-dev mmctl --local plugin enable com.pardis.muchat
  ```

### Testing

- Run unit tests:

  ```bash
  make test
  ```

- Run end-to-end tests:

  ```bash
  make e2e
  ```

## License

This project is licensed under the [MIT License](LICENSE).
