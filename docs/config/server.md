---
title: Server Configuration
createTime: 2025/04/01 00:08:53
permalink: /config/server
---

CalendarAPI uses a single `config.yaml` file to define all runtime settings, including server ports, subscribed calendars, and event rules.

This page describes the configuration options available in the `server` section of the config.

---

## Config File Locations

CalendarAPI searches for `config.yaml` in the following order:

<FileTree>

- ./ (local directory)
  - config.yaml
  - calendarapi (binary)
- ~/
  - .config/
    - calendarapi/
      - config.yaml
- /data/
  - config.yaml

</FileTree>

Place your config file in one of these locations to have it automatically picked up.

---

## Server Parameters

These settings control the behavior of the CalendarAPI server.

| Key        | Type            | Required | Description                                                                                  |
|------------|-----------------|----------|----------------------------------------------------------------------------------------------|
| `host`     | string           | no       | The address to bind to (in server mode) or connect to (in client mode).                     |
| `httpPort` | integer          | no       | Port to expose the REST API. Default is `8099`. Requires restart if changed.                |
| `grpcPort` | integer          | no       | Port to expose the gRPC API. Default is `50051`. Requires restart if changed.               |
| `debug`    | boolean          | no       | Enables verbose debug logging. Default is `false`.                                          |
| `refresh`  | time.Duration    | no       | How often CalendarAPI refreshes calendars. Default is `30m`. Accepts Go duration strings.   |

---

## Example Configuration (Server Mode)

```yaml
server:
  host: ""
  httpPort: 8080
  grpcPort: 50051
  debug: false
  refresh: 5m
```

---

## Example Configuration (Client Mode)

When using the `calendarapi` CLI as a client, only the `host` and `debug` options are needed:

```yaml
server:
  host: "homeassistant.local"
  debug: false
```

---

## Notes

- Changes to `host`, `httpPort`, or `grpcPort` require restarting the CalendarAPI process.
- `refresh` accepts Go-style durations such as `5m`, `1h`, or `30s`.
- If no `host` is set in client mode, you must use the `--server` flag to specify a target server.
