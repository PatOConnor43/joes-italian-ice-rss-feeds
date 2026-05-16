package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ============ Embedded Data Structures ============

type EmbeddedTable struct {
	Collection CollectionData `json:"collection"`
	Items      ItemsData      `json:"items"`
}

type CollectionData struct {
	TableID int    `json:"table_id"`
	Name    string `json:"table_title"`
}

func (c *CollectionData) UnmarshalJSON(data []byte) error {
	type Alias CollectionData
	aux := &struct {
		TableID interface{} `json:"table_id"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	switch v := aux.TableID.(type) {
	case float64:
		c.TableID = int(v)
	case string:
		fmt.Sscanf(v, "%d", &c.TableID)
	}
	return nil
}

type ItemsData struct {
	Columns []ColumnData `json:"columns"`
	Rows    []RowData    `json:"rows"`
}

func (i *ItemsData) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "{}" {
		return nil
	}
	type Alias ItemsData
	return json.Unmarshal(data, (*Alias)(i))
}

type ColumnData struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Format string `json:"format"`
}

type RowData struct {
	RecordID string       `json:"record_id"`
	Content  ContentMap   `json:"content"`
}

// ContentMap handles both array [{"value":"..."}] and object {"0":{"value":"..."}} formats
type ContentMap map[string]any

func (c *ContentMap) UnmarshalJSON(data []byte) error {
	*c = make(map[string]any)
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err == nil {
		if _, hasType := obj["type"]; hasType {
			*c = obj
			return nil
		}
		for _, v := range obj {
			if mv, ok := v.(map[string]any); ok {
				if _, hasType := mv["type"]; hasType {
					*c = obj
					return nil
				}
			}
		}
	}
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err == nil {
		for _, item := range arr {
			if colID, ok := item["column_id"]; ok {
				switch v := colID.(type) {
				case float64:
					(*c)[fmt.Sprintf("%d", int(v))] = item
				}
			}
		}
		return nil
	}
	return fmt.Errorf("unknown content format")
}

// ============ Flavor Data ============

type StoreFlavors struct {
	TableID      int
	StoreName    string
	Location     string
	Date         string
	LatestUpdate string
	StoreHours   string
	Flavors      []string
	OrderURL     string
	ScrapeTime   time.Time
}

// ============ RSS Feed ============

type RSSFeed struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Language    string `xml:"language"`
	Copyright   string `xml:"copyright"`
	Generator   string `xml:"generator"`
	LastBuild   string `xml:"lastBuildDate"`
	PubDate     string `xml:"pubDate"`
	TTL         string `xml:"ttl"`
	Item        []Item `xml:"item"`
}

type Item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

// ============ Scraper ============

type Scraper struct {
	client  *http.Client
	baseURL string
}

func NewScraper() *Scraper {
	return &Scraper{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://joesice.com",
	}
}

func (s *Scraper) FetchPage(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://joesice.com/")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func ExtractEmbeddedTables(html string) ([]EmbeddedTable, error) {
	pattern := regexp.MustCompile(`window\.tablesomeTables\s*=\s*(\[[\s\S]*?\])\s*;`)
	matches := pattern.FindStringSubmatch(html)
	if len(matches) < 2 {
		return nil, fmt.Errorf("could not find tablesomeTables in page")
	}
	var tables []EmbeddedTable
	if err := json.Unmarshal([]byte(matches[1]), &tables); err != nil {
		return nil, err
	}
	return tables, nil
}

func ParseStoreFlavors(tables []EmbeddedTable) map[int]*StoreFlavors {
	flavorTables := make(map[int]EmbeddedTable)
	timeTables := make(map[int]EmbeddedTable)

	for _, table := range tables {
		switch table.Collection.TableID {
		case 20683, 20697:
			flavorTables[table.Collection.TableID] = table
		case 20752, 20754:
			timeTables[table.Collection.TableID] = table
		}
	}

	stores := make(map[int]*StoreFlavors)
	for tableID, table := range flavorTables {
		store := &StoreFlavors{
			TableID:    tableID,
			ScrapeTime: time.Now(),
		}

		for _, row := range table.Items.Rows {
			if val := extractValue(row.Content); val != "" {
				store.Flavors = append(store.Flavors, val)
			}
		}

		switch tableID {
		case 20683:
			store.StoreName = "Joe's Italian Ice - Anaheim"
			store.Location = "Anaheim, CA"
			store.OrderURL = "https://order.online/store/joe's-italian-ice-anaheim-2285412/?hideModal=true&pickup=true&redirected=true"
			if tt, ok := timeTables[20752]; ok {
				mergeTimeInfo(tt, store)
			}
		case 20697:
			store.StoreName = "Joe's Italian Ice - Tempe"
			store.Location = "Tempe, AZ"
			store.OrderURL = "https://order.online/store/joe's-italian-ice-tempe-353993/?delivery=true&hideModal=true&redirected=true"
			if tt, ok := timeTables[20754]; ok {
				mergeTimeInfo(tt, store)
			}
		}

		store.Flavors = deduplicateFlavors(store.Flavors)
		stores[tableID] = store
	}
	return stores
}

func mergeTimeInfo(table EmbeddedTable, store *StoreFlavors) {
	if len(table.Items.Columns) > 0 {
		colName := table.Items.Columns[0].Name
		if strings.Contains(colName, "January") || strings.Contains(colName, "February") ||
			strings.Contains(colName, "March") || strings.Contains(colName, "April") ||
			strings.Contains(colName, "May") || strings.Contains(colName, "June") ||
			strings.Contains(colName, "July") || strings.Contains(colName, "August") ||
			strings.Contains(colName, "September") || strings.Contains(colName, "October") ||
			strings.Contains(colName, "November") || strings.Contains(colName, "December") {
			store.Date = colName
		}
	}
	for _, row := range table.Items.Rows {
		value := extractValue(row.Content)
		if strings.HasPrefix(value, "Latest Update:") {
			store.LatestUpdate = value
		}
		if strings.HasPrefix(value, "Store Hours:") {
			store.StoreHours = value
		}
	}
}

func extractValue(content ContentMap) string {
	if val, ok := content["value"].(string); ok {
		return val
	}
	if val, ok := content["html"].(string); ok {
		return val
	}
	for _, v := range content {
		if cv, ok := v.(map[string]any); ok {
			if val, ok := cv["value"].(string); ok {
				return val
			}
			if val, ok := cv["html"].(string); ok {
				return val
			}
		}
	}
	return ""
}

func (s *Scraper) FetchAllFlavors() ([]*StoreFlavors, error) {
	html, err := s.FetchPage(s.baseURL)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}

	tables, err := ExtractEmbeddedTables(html)
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}

	storeMap := ParseStoreFlavors(tables)
	var stores []*StoreFlavors
	for _, store := range storeMap {
		stores = append(stores, store)
	}
	return stores, nil
}

// ============ Helpers ============

func deduplicateFlavors(flavors []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, f := range flavors {
		n := strings.TrimSpace(f)
		if n != "" && !seen[n] {
			seen[n] = true
			result = append(result, n)
		}
	}
	return result
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

func buildDescription(store *StoreFlavors) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<h3>%s</h3>", escapeHTML(store.Location)))
	sb.WriteString(fmt.Sprintf("<p><strong>Date:</strong> %s</p>", escapeHTML(store.Date)))
	sb.WriteString(fmt.Sprintf("<p><strong>Update:</strong> %s</p>", escapeHTML(store.LatestUpdate)))
	sb.WriteString(fmt.Sprintf("<p><strong>Hours:</strong> %s</p>", escapeHTML(store.StoreHours)))
	sb.WriteString("<p><strong>Flavors:</strong></p><ul>")
	for _, flavor := range store.Flavors {
		sb.WriteString(fmt.Sprintf("<li>%s</li>", escapeHTML(flavor)))
	}
	sb.WriteString("</ul>")
	if store.OrderURL != "" {
		sb.WriteString(fmt.Sprintf("<p><a href=\"%s\">Order Online</a></p>", store.OrderURL))
	}
	return sb.String()
}

func writeRSS(stores []*StoreFlavors, dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	for _, store := range stores {
		item := Item{
			Title:       fmt.Sprintf("Daily Flavors - %s (%s)", store.StoreName, store.Date),
			Link:        store.OrderURL,
			Description: buildDescription(store),
			PubDate:     store.ScrapeTime.UTC().Format(time.RFC1123Z),
			GUID:        fmt.Sprintf("flavors-%d-%s", store.TableID, store.ScrapeTime.Format("20060102")),
		}

		rss := &RSSFeed{
			Version: "2.0",
			Channel: Channel{
				Title:       store.StoreName + " - Daily Flavors",
				Link:        store.OrderURL,
				Description: fmt.Sprintf("Daily ice cream and Italian ice flavors at %s", store.Location),
				Language:    "en-us",
				Copyright:   fmt.Sprintf("Copyright %d Joe's Italian Ice", store.ScrapeTime.Year()),
				Generator:   "joes-italian-ice-rss-feeds",
				LastBuild:   store.ScrapeTime.UTC().Format(time.RFC1123Z),
				PubDate:     store.ScrapeTime.UTC().Format(time.RFC1123Z),
				TTL:         "60",
				Item:        []Item{item},
			},
		}

		data, err := xml.MarshalIndent(rss, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}

		filename := strings.ToLower(strings.ReplaceAll(store.Location, ", ", "-")) + ".xml"
		path := filepath.Join(dir, filename)
		if err := os.WriteFile(path, append([]byte(xml.Header), data...), 0644); err != nil {
			return fmt.Errorf("write %s: %w", filename, err)
		}
	}
	return nil
}

// ============ Main ============

func main() {
	scraper := NewScraper()
	fmt.Print("Fetching daily flavors... ")

	stores, err := scraper.FetchAllFlavors()
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%d store(s), %d total flavors\n", len(stores), totalFlavors(stores))

	if err := writeRSS(stores, "rss"); err != nil {
		fmt.Printf("error writing RSS: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Written to rss/*.xml")
	for _, store := range stores {
		loc := strings.ToLower(strings.ReplaceAll(store.Location, ", ", "-"))
		fmt.Printf("  %s -> rss/%s.xml\n", store.StoreName, loc)
	}
}

func totalFlavors(stores []*StoreFlavors) int {
	n := 0
	for _, s := range stores {
		n += len(s.Flavors)
	}
	return n
}
