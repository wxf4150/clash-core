# Reload and Restart Features

## Overview

Clash now supports graceful configuration reload and application restart through both signal handling and REST API endpoints.

## Signal Handling

### SIGHUP - Configuration Reload

Send SIGHUP signal to reload configuration from disk without restarting the application:

```bash
# Find the process ID
ps aux | grep clash

# Send SIGHUP signal
kill -HUP <PID>
```

This will:
- Reload the configuration file from disk
- Apply the new configuration without restarting listeners
- Log success or error messages

### SIGUSR1 - Application Restart

Send SIGUSR1 signal to trigger application restart:

```bash
kill -USR1 <PID>
```

**Note:** This will exit the application. You should use a process manager (systemd, docker, supervisord) to automatically restart the application.

## REST API Endpoints

### POST /configs/reload

Reload configuration from disk.

**Request:**
```bash
curl -X POST http://127.0.0.1:9090/configs/reload \
  -H "Authorization: Bearer YOUR_SECRET"
```

**Response:**
- 204 No Content on success
- 500 Internal Server Error with error message on failure

### POST /configs/restart

Trigger application restart.

**Request:**
```bash
curl -X POST http://127.0.0.1:9090/configs/restart \
  -H "Authorization: Bearer YOUR_SECRET"
```

**Response:**
```json
{
  "message": "restarting"
}
```

The application will exit after sending the response. A process manager should be configured to automatically restart it.

## Example with systemd

Add `Restart=always` to your systemd service file:

```ini
[Unit]
Description=Clash daemon
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/clash -d /etc/clash
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Then you can use either signals or API endpoints to trigger restart, and systemd will automatically restart the application.

## Example with Docker

Use restart policy in docker-compose.yml:

```yaml
version: '3'
services:
  clash:
    image: clash
    restart: always
    volumes:
      - ./config.yaml:/root/.config/clash/config.yaml
```

## Use Cases

- **Configuration Reload (SIGHUP or /configs/reload)**: Update rules, proxies, or other settings without interrupting existing connections
- **Application Restart (SIGUSR1 or /configs/restart)**: Full restart when needed (e.g., after binary updates)
