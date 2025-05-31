---
title: Home Assistant Integration
createTime: 2025/04/01 00:08:53
permalink: /config/home_assistant
---

CalendarAPI exposes calendar and status data over a simple REST API, making it easy to integrate with Home Assistant using [RESTful sensors](https://www.home-assistant.io/integrations/sensor.rest/), [template sensors](https://www.home-assistant.io/integrations/template/), and [REST commands](https://www.home-assistant.io/integrations/rest_command/).

This guide walks through:

- Monitoring your current meeting status using sensors
- Setting and clearing custom status messages from automations

---

## Sensor Integration

The following example sets up:

| Entity                              | Description |
|-------------------------------------|-------------|
| `sensor.current_meeting`            | Shows the title of the current event from CalendarAPI. Empty if no event is active. |
| `binary_sensor.meeting_in_progress` | Indicates whether a meeting is currently happening based on `sensor.current_meeting`. |
| `sensor.current_epd_status`         | Reflects the current custom status (e.g., "Working remotely") set via the CalendarAPI server. |

### Prerequisites

- A running instance of CalendarAPI accessible from Home Assistant
- Working network connection between both systems

::: tip
The examples below assume your CalendarAPI server is available at `http://192.168.0.62:8099`.  
Adjust the address as needed for your setup.
:::

### Sensor Configuration

Add the following to your Home Assistant `configuration.yaml`:

```yaml
template:
  binary_sensor:
    - unique_id: meeting_in_progress
      name: "Meeting In Progress"
      state: >
        {% if is_state('sensor.current_meeting', '') %}
          off
        {% else %}
          on
        {% endif %}
      availability: >
        {{ states('sensor.current_meeting') not in ['unavailable', 'unknown'] }}

sensor:
  - unique_id: current_meeting
    platform: rest
    name: "Current Meeting"
    resource: http://192.168.0.62:8099/calendar/current
    value_template: "{{ value_json.title }}"

  - unique_id: current_epd_status
    platform: rest
    name: "Current EPD Status"
    resource: http://192.168.0.62:8099/status?calendar=all
    value_template: "{{ value_json.title }}"
```

---

## Sending Custom Status Updates

CalendarAPI allows you to set rich, contextual status messages via a REST API. This can be used to show dynamic messages on e-Paper displays or dashboards — such as _“In a meeting”_ or _“Be back at 13:00”_.

### Supported Fields

| Field         | Description                                      |
|---------------|--------------------------------------------------|
| `icon`        | Icon name (e.g., from Material Design Icons)     |
| `icon_size`   | Integer representing icon display size           |
| `title`       | Short message (e.g., "Lunch Break")              |
| `description` | Optional longer status message                   |

### Command Configuration

Add these commands to your `configuration.yaml`:

```yaml
rest_command:
  set_epd_status:
    url: http://192.168.0.62:8099/status
    method: post
    content_type: application/json
    payload: >
      {
        "calendar_name": "all",
        "status": {
          "icon": "{{ icon }}",
          "icon_size": {{ icon_size }},
          "title": "{{ title }}",
          "description": "{{ description }}"
        }
      }

  clear_epd_status:
    url: http://192.168.0.62:8099/status
    method: delete
    content_type: application/json
    payload: >
      {
        "calendar_name": "all"
      }
```

---

## Example Scripts

To set a custom status:

```yaml
script:
  lunch_break_status:
    alias: "Set Lunch Break Status"
    sequence:
      - service: rest_command.set_epd_status
        data:
          icon: "mdi:food"
          icon_size: 3
          title: "Lunch Break"
          description: "Be back at 13:00"
```

To clear it:

```yaml
script:
  clear_epd:
    alias: "Clear EPD Status"
    sequence:
      - service: rest_command.clear_epd_status
```

---

## Troubleshooting

- Use **Developer Tools → Services** in Home Assistant to manually test `rest_command.set_epd_status`.
- Use the REST sensors above to confirm status updates are reflected in CalendarAPI.
- Check CalendarAPI logs if updates don’t appear — malformed JSON or incorrect field names are common issues.

---

For more information on configuration options, refer to:

- [Calendar Configuration](/config/calendars)
- [Server Settings](/config/server)
- [Rules Engine](/config/rules)
