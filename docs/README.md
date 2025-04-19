---
home: true
title: Home

actions:
    - text: Get Started
      link: /guide/
    - text: Download
      link: https://github.com/SpechtLabs/CalendarAPI/releases
      type: secondary
---

CalendarAPI is a service that parses iCal files and exposes their content via gRPC or a REST API. It uses Viper for configuration, which supports runtime reloads.

## ⚙️ Features

- ✅ Parse iCal (.ics) files from **URLs or local files**
- ✅ Exposes events via **REST** and **gRPC** APIs
- ✅ Built-in **rule engine** for relabeling, filtering, and skipping events
- ✅ Supports **hot configuration reloads** (with [Viper](https://github.com/spf13/viper))
- ✅ [HomeAssistant Add-On] to easily host CalendarAPI on your Home Assistant

<ClientOnly>
    <Contributors repo="SpechtLabs/CalendarAPI" />
    <Releases repo="SpechtLabs/CalendarAPI" />
</ClientOnly>