![Athena logo](resource/logo.png)<br>
Athena is a lightweight AO2 server written in Go.<br>
Athena was created with a few core principles in mind:
* Being fast and efficient: Athena is built on concurrency, leveraging the full power of modern multi-core CPUs.
* Being simple to setup and configure.
* Having a more minimalist feature list, retaining vital and often used features while discarding unnessecary bloat.

## Features
* WebAO support
* WebSocket Secure (WSS) support for encrypted connections via Cloudflare
* Concurrent handling of client connections
* A moderator user system with configurable roles to set permissions
* A robust command system
* Easy to understand configuration using [TOML](https://toml.io/en/)
* Passwords stored using bcrypt
* A CLI command parser, allowing basic commands to be run without connecting with a client
* A privacy-oriented logging system, allowing for easy moderation while maintaining user privacy
* Testimony recorder

## Quick Start
Download the [latest release](https://github.com/MangosArentLiterature/Athena/releases/latest), extract into a folder of your chosing.<br>
Rename `config_sample` to `config` and modify the configuration files.<br>
Run the executable and setup your initial moderator account with `mkusr`.

## Configuration
By default, athena looks for its configuration files in the `config` directory.<br>
If you'd like to store your configuration files elsewhere, you can pass the `-c` flag on startup with the path to your configuration directory.<br>
CLI input can be disabled with `-nocli`

### WebSocket Secure (WSS) Configuration
To enable secure WebSocket connections for Cloudflare and other proxies:

**Option 1: Using a Reverse Proxy (Recommended for Cloudflare)**
1. In `config.toml`, set:
   ```toml
   enable_webao_secure = true
   webao_secure_port = 443  # or your preferred port
   ```
2. Leave `tls_cert_path` and `tls_key_path` empty
3. Configure your reverse proxy (Cloudflare, nginx, etc.) to handle TLS termination
4. The proxy forwards to your server via regular HTTP/WebSocket

**Option 2: Direct TLS (Server Handles Encryption)**
1. Obtain TLS certificates (e.g., from Let's Encrypt)
2. In `config.toml`, set:
   ```toml
   enable_webao_secure = true
   webao_secure_port = 443  # or your preferred port
   tls_cert_path = "/path/to/your/certificate.crt"
   tls_key_path = "/path/to/your/private.key"
   ```
3. Server handles TLS encryption directly

When advertising to the master server, your server will be listed with `wss://` support regardless of which option you choose.