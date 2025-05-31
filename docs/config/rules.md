---
title: Rules Engine
createTime: 2025/04/01 00:08:53
permalink: /config/rules
---

Rules allow you to filter, skip, or relabel calendar events before they are exposed via the CalendarAPI REST and gRPC interfaces.

Rules are evaluated **in the order they are defined**, and **only the first matching rule is applied** per event. This makes rule order critical when combining filters and catch-all patterns.

## Rule Structure

Each rule consists of:

| Key            | Type     | Description |
|----------------|----------|-------------|
| `name`         | string   | A descriptive name for the rule (used for logging/debugging) |
| `key`          | string   | The field to match against (`title`, `busy`, `all_day`, or `*` for wildcard matching) |
| `contains`     | list     | A list of substrings or values to match against the selected key |
| `skip`         | boolean  | If `true`, the matching event will be excluded from all API responses |
| `relabelConfig`| object   | Optional — used to rewrite message, icon, or mark importance |

Only one of `skip` or `relabelConfig` should typically be used per rule.

## Relabeling Events

You can relabel events by changing their title (`message`), setting an `icon`, or marking them as `important`.

```yaml
rules:
  - name: "1:1s"
    key: "title"
    contains:
      - "1:1"
    relabelConfig:
      message: "1:1"
      important: true
```

This rule matches any event whose title contains "1:1", and replaces the display message while flagging it as important.

## Skipping Events

If a rule contains `skip: true`, matching events are filtered out and will not appear in any API responses.

### Example: Skipping All-Day and Free Events

```yaml
rules:
  - name: "Skip all day events"
    key: "all_day"
    contains:
      - "true"
    skip: true

  - name: "Skip non-blocking events"
    key: "busy"
    contains:
      - "Free"
    skip: true
```

## Wildcard Matching

To create a fallback rule that applies to all events not matched by earlier rules, use `*` for both `key` and `contains`.

```yaml
rules:
  - name: "Allow everything else"
    key: "*"
    contains:
      - "*"
    relabelConfig:
      important: false
```

Wildcard rules are typically placed at the **end** of the rule list to act as a catch-all.

## Field Reference

You can use the following values for `key`:

| Key       | Description                                  |
|-----------|----------------------------------------------|
| `title`   | The event title (summary/subject)            |
| `busy`    | Whether the event is marked "Busy" or "Free" |
| `all_day` | Whether the event is an all-day event        |
| `*`       | Wildcard — applies to all fields             |

## Tips

- Define the most specific rules **first**, and catch-all rules **last**.
- Use `skip: true` for events you don't want to expose at all.
- Use `relabelConfig` to control how events appear to consumers of the API.
