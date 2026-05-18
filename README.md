# Joe's Italian Ice RSS Feeds

Automatically generated RSS feeds for Joe's Italian Ice locations, updated daily via GitHub Actions.

## Feeds Available

### Location Feeds
Get all daily flavor updates for a specific location:

- **Anaheim, CA**: `https://raw.githubusercontent.com/PatOconnor43/joes-italian-ice-rss-feeds/master/rss/anaheim-ca.xml`
- **Tempe, AZ**: `https://raw.githubusercontent.com/PatOconnor43/joes-italian-ice-rss-feeds/master/rss/tempe-az.xml`

### Individual Flavor Feeds
Subscribe to notifications when a specific flavor is available at a location. Feeds are organized by location and flavor name, e.g.:
- `rss/anaheim-ca-strawberry.xml`
- `rss/tempe-az-raspberry-tart.xml`

## Quick Subscribe - Import All Feeds at Once

The easiest way to subscribe to all feeds is to import the OPML file into your RSS reader.

### What is OPML?

OPML (Outline Processor Markup Language) is a standard format that lets you subscribe to multiple feeds at once. Instead of adding each feed individually, you can import a single file and get all the feeds in one go.

### How to Subscribe

1. **Copy the OPML URL**:
   ```
   https://raw.githubusercontent.com/PatOconnor43/joes-italian-ice-rss-feeds/master/rss/feeds.opml
   ```

2. **Import into your RSS reader**:
   - **Feedly**: Click the Menu icon → "Add content" → "Import OPML" → Paste the URL
   - **Inoreader**: Settings → "Import & Export" → "Import OPML" → Paste the URL
   - **NewsBlur**: Go to "Organize" → "Import" → Paste the URL
   - **Other readers**: Look for an "Import OPML" or "Subscribe" option in your reader's settings

3. **Organize**: Once imported, the feeds will be organized into two folders:
   - **Location Feeds** - All daily flavors for each location
   - **Individual Flavor Feeds** - Availability notifications for specific flavors

## Manual Subscription

If you prefer to add feeds manually, you can find all available RSS feeds in the `rss/` directory. Each file is a valid RSS feed URL.

## How It Works

- Scrapes the Joe's Italian Ice website daily for updated flavor information
- Generates location-level feeds (all flavors at a location)
- Generates individual flavor feeds (availability of a specific flavor)
- Automatically commits updates to this repository
- Updates the OPML file with all available feeds

## Update Schedule

Feeds are updated automatically every day at **20:00 UTC** via GitHub Actions.

