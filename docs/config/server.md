# Server

Calendar API uses a `yaml` configuration file allowing you to specify your server-settings, iCal calendars to subscribe,
as well as rules that are applied to calendar events.

::: tip
You can place your `config.yaml` file in one of the following directories, that CalendarAPI looks up in the same order:

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

:::

## Server Config

In the `server` section of your config file, you can specify the following parameters:

| Key        | Type            | Description                                                                                                                                                                               |
|:-----------|:----------------|:------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `host`     | `string`        | The Server Address to bind to (when using `serve`) or connect to (when using the CLI as client). **Note:** Does not support hot-reloading of the config during runtime. Requires restart. |
| `httpPort` | `int`           | The REST API Port (_Default:_ `8099`). **Note:** Does not support hot-reloading of the config during runtime. Requires restart.                                                           |
| `grpcPort` | `int`           | The gRPC API Port (_Default:_ `50051`). **Note:** Does not support hot-reloading of the config during runtime. Requires restart.                                                          |
| `debug`    | `int`           | Enable debug settings (_Default:_ `false`)                                                                                                                                                |
| `refresh`  | `time.Duration` | How often the iCal calendars are refreshed (_Default:_ `30m`)                                                                                                                             | 

### Example (Server)

```yaml
server:
  host: ""
  httpPort: 8080
  grpcPort: 50051
  debug: false
  refresh: 5m
```

### Example (Client)
```yaml
server:
  host: "homeassistant.local"
  debug: false
```
