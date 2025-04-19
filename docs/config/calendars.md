# Calendars

The `calendars` config specifies an array of calendars you want to subscribe to.
Calendars can either be read from an URL, or from the file-system. Each calendar needs an unique name

| Key        | Type         | Description                                           |
|:-----------|:-------------|:------------------------------------------------------|
| `calendars` | `[]calendar` | Instances of of [`calendar`](#calendar-config) items. |

## Calendar Config

| Key    | Type   | Description                                                                             |
|:-------|:-------|:----------------------------------------------------------------------------------------|
| `from` | `enum` | Can be either `file` or `url` to specify if the ical is read from disk, or from the web |
| `ical` | `string` | Either the path to the local ical (*.ics) calendar file, or the URL to the calendar you wish to susbscribe to |

## Example Calendars

```yaml
calendars:
  calendar1:
    from: file
    ical: /Users/cedi/.config/calendars/calender1.ics
  calendar2:
    from: url
    ical: www.example.com/calendar/calendar.ics
```
