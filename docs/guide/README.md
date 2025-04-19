# Introduction

CalendarAPI was originally built to power an e-Paper-based meeting room display, running on an ESP32
Since the device had limited memory and parsing iCal calendars in Arduino C would’ve been a major hassle, 
CalendarAPI offloaded that complexity into a dedicated Go service that exposes the parsed calendar data 
over both REST and gRPC APIs.

While it started as a backend utility, CalendarAPI quickly proved useful beyond its original scope. 
It became a handy CLI tool to check daily calendar events via `calendarapi get calendar`, 
and even got a [HomeAssistant Add-On](https://github.com/SpechtLabs/homeassistant-addons/tree/main/calendar_api) for seamless integration into smart home setups. 
With [RESTful sensors](https://www.home-assistant.io/integrations/sensor.rest/) and [RESTful commands](https://www.home-assistant.io/integrations/rest_command/), it can power HomeAssistant sensors or display contextual info on the e-Paper device.

CalendarAPI also supports custom status messages per calendar, making it easy to show tailored messages on 
displays throughout the day—whether it’s “In a meeting” or “Out for lunch.”

At its core, CalendarAPI exists to make calendar data more accessible, automatable, 
and adaptable to embedded devices and home automation systems.
