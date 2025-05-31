---
title: Calendars
createTime: 2025/04/01 00:08:53
permalink: /config/calendars
---

The `calendars` section defines which iCal calendar sources CalendarAPI should load.  
Each calendar must have a unique name and specify where its `.ics` file should be retrieved from.

Calendars can be loaded from either:

- A **local file path** (e.g., a `.ics` file on disk)
- A **remote URL** (e.g., a public or private iCal feed)

## Configuration Structure

```yaml
calendars:
  - name: work
    from: url
    ical: "https://example.com/calendar.ics"

  - name: personal
    from: file
    ical: "/etc/calendarapi/personal.ics"
```

## Field Reference

| Field    | Type     | Required | Description                                                                 |
|----------|----------|----------|-----------------------------------------------------------------------------|
| `name`   | string   | yes      | Unique identifier for the calendar source. Used in status updates and API calls. |
| `from`   | string   | yes      | Must be either `file` or `url`, indicating how to load the calendar.       |
| `ical`   | string   | yes      | Path to a local `.ics` file or a full URL to a remote calendar feed.       |

::: note

- Calendar names must be unique.
- Remote URLs must be accessible by the CalendarAPI server.
- Local paths must be readable by the process running CalendarAPI.

:::

## Example Use Cases

### A Local File-Based Calendar

```yaml
calendars:
  - name: internal
    from: file
    ical: /data/calendars/meetings.ics
```

### A Public Google Calendar

```yaml
calendars:
  - name: team
    from: url
    ical: "https://calendar.google.com/calendar/ical/team%40example.com/private-uuid/basic.ics"
```
