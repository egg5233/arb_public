# Binance CMS API Documentation

Source: https://developers.binance.com/docs/

---

Announcements

# Changelog

## 2025-07-21

- Added announcement subscription feature

Copyright © 2026 Binance.

---

Announcements

# Announcements

- When Binance publishes announcements in English (language code: `en`), subscribed users will receive push notifications.

## Topic

- `com_announcement_en`

## Response Content

- Due to the length of the announcement, the example uses `...` to indicate omitted parts.

#### Full Response Example

```json
{
  "type": "DATA",
  "topic": "com_announcement_en",
  "data": "{\"catalogId\":161,\"catalogName\":\"Delisting\",\"publishDate\":1753257631403,\"title\":\"Notice of...\",\"body\":\"This is...\",\"disclaimer\":\"Trade on-the-go...\"}"
}
```

#### Announcement Content Description

- The detailed announcement content is contained in the `data` field. After unescaping the string, the structure is as follows:

```javascript
{
    "catalogId": 161,                     --Catalog ID
    "catalogName": "Delisting",           --Catalog Name
    "publishDate": 1753257631403,         --Publish timestamp (milliseconds)
    "title": "Notice of...",              --Announcement title
    "body": "This is...",                 --Announcement body
    "disclaimer": "Trade on-the-go..."    --Disclaimer
}
```

 

Copyright © 2026 Binance.

---

Announcements

# Changelog

## 2025-07-21

- Added announcement subscription feature

Copyright © 2026 Binance.

---

