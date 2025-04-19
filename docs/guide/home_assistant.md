# HomeAssistant Integration

CalendarAPI exposes your calendar and status data over a simple REST API, making it easy to consume 
in HomeAssistant using REST and Template sensors. 

This guide will walk you through setting up two REST sensors and a template binary sensor to display 
your current calendar event and whether you are currently in a meeting.

## Example [RESTful sensors](https://www.home-assistant.io/integrations/sensor.rest/)

| Entity                              | Description                                                                                                                                                                                        |
|:------------------------------------|:---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `sensor.current_meeting`            | Fetches the title of the current event from your CalendarAPI. If there's no ongoing event, it returns an empty string.                                                                             |
| `binary_sensor.meeting_in_progress` | Evaluates whether `sensor.current_meeting` has a value. If it does, the binary sensor is `on` (a meeting is in progress); otherwise, it's `off`.                                                   |
| `sensor.current_epd_status`         | Reads the current custom status from the CalendarAPI server. This can be used to display user-defined messages (e.g., "Out for lunch", "Working remotely") on your dashboards or e-Paper displays. |


### Prerequisites

* A running instance of CalendarAPI on your network.
* HomeAssistant with access to your CalendarAPI instance (e.g., same network or routed connection).

::: note
This example assumes your CalendarAPI is accessible at http://192.168.0.62:8099.

Update the IP address and port to match your setup.
:::

### Sensor Configuration

Add the following to your HomeAssistant `configuration.yaml` file:

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

## Example [RESTful commands](https://www.home-assistant.io/integrations/rest_command/)

CalendarAPI supports setting custom status messages for each calendar. These status messages can be used to show rich, contextual information (e.g., icons, titles, descriptions) on devices like your e-Paper meeting room display.

HomeAssistant’s [rest_command](https://www.home-assistant.io/integrations/rest_command/) integration allows you to trigger these updates with automations, scripts, or manually via UI.

### Status Payload Structure

Each custom status supports the following fields:

| Field         | Description                                           |
|:--------------|:------------------------------------------------------|
| `icon`        | A name of an icon to be displayed.                    |
| `icon_size`   | An integer representing the display size of the icon. |
| `title`       | A short title or message (e.g., "Ad-Hoc Meeting").    |
| `description` | _Optional:_ A longer status message.                  |

### Command Configuration

Add the following to your HomeAssistant `configuration.yaml` file:

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

::: Note
Replace `192.168.0.62:8099` with the address of your CalendarAPI server if it differs.
:::

### Usage

You can trigger a status update using a script like this:

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

And clear it with 

```yaml
script:
  clear_epd:
    alias: "Clear EPD Status"
    sequence:
      - service: rest_command.clear_epd_status
```

## HomeAssistant Debugging Tipps

Use the **Developer Tools → Services** tab to manually trigger `rest_command.set_epd_status`.

Use the REST sensor from the previous section to confirm the updated status is reflected.

Check your CalendarAPI logs if nothing updates — it may be an input formatting issue.