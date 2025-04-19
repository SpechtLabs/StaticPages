# Commands

The CalendarAPI Binary can be used both, as a CLI app, and as the server component in your infrastructure.

## Server

The `serve` command will read in the [provided iCal calendar from your config](../config/calendars.md) and then serve
them via the gRPC and REST APIs on the ports configured in your [server config](../config/server.md)

## Client

If you wish to use the `clusterapi` command as a client, for example to fetch the calendar for the day, or to set a custom
status, then simply set the [server URL in the configuration file](../config/server.md) or specify the `--server`
parameter on all calls to `clusterapi`.

The `clusterapi` command follows a similar structure to other, modern, CLI applications:

`clusterapi [VERB] [NOUN]`

See the below command structure of `clusterapi` or use `clusterapi --help` to find more information

<FileTree>

- get
    - status
    - calendar
- clear
    - status
    - calendar
- set
    - status

</FileTree>

### Tab Completion

`clusterapi` supports auto-completion for all major shells:

* bash
* fish
* powershell
* zsh

For example, if you wish to enable tab-completion for zsh, you can add

```bash
if [[ $(command -v calendarapi) ]]; then
  eval "$(calendarapi completion zsh)"
fi
```

to your `~/.zshrc`