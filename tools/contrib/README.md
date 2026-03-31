# contrib — Community Tools

These tools are API integrations maintained on a best-effort basis.
APIs change — if a tool breaks, PRs welcome.

## Usage

import "github.com/saeedalam/agnogo/tools/contrib"

agent := agnogo.Agent("...", agnogo.Tools(contrib.HackerNews()...))

## Available Tools

### Communication
- Discord(token) — messaging, channels
- Telegram(token, chatID) — send/edit/delete messages
- WhatsApp(accessToken, phoneNumberID) — text, images, location

### Project Management
- Jira(serverURL, username, token) — issues, search, comments
- Notion(apiKey, databaseID) — pages, search
- Linear(apiKey) — issues, teams (GraphQL)
- GitLab(token) — projects, MRs, issues

### Search & Data
- HackerNews() — top stories, user details
- DuckDuckGoSearch() — enhanced web/news search
- Arxiv() — paper search
- Reddit(clientID, clientSecret, userAgent) — posts, subreddits
- YouTube() — video data, captions
- YFinance() — stock prices, company info

### Media
- Giphy(apiKey) — GIF search
- Unsplash(accessKey) — photo search

### Google Services
- GoogleMaps(apiKey) — places, geocoding, directions
- GoogleCalendar(accessToken) — events CRUD
- GoogleSheets(accessToken) — read/write spreadsheets
- GoogleSearch(apiKey, cx) — custom search

### Weather
- OpenWeather(apiKey) — current weather, forecast

### Utilities
- Archive() — tar.gz create/extract
- Base64() — encode/decode
- Crypto() — AES-256-GCM encrypt/decrypt
- CronTool() — parse cron expressions
- Diff() — text diff
- DNS() — lookup, MX, TXT
- Env(allowlist) — environment variables
- ImageTool() — info, resize, crop, convert
- Markdown() — strip to plain text
- MetricsTool() — Prometheus format
- PDFTool() — info, text extraction
- Semver() — parse/compare versions
- TCP() — port check, scan
- TemplateTool() — Go template rendering
- UUID() — generate v4
- XML() — parse/format, XPath queries
- YAML() — full parser with anchors/aliases
