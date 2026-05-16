package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// ============ Embedded Data Structures ============

// EmbeddedTable represents a table in the embedded JavaScript data
type EmbeddedTable struct {
	Collection CollectionData `json:"collection"`
	Items      ItemsData      `json:"items"`
}

// CollectionData holds table metadata
type CollectionData struct {
	TableID    int    `json:"table_id"`
	TableTitle string `json:"table_title"`
}

// UnmarshalJSON handles table_id as string or int
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

// ItemsData holds columns and rows
type ItemsData struct {
	Columns []ColumnData `json:"columns"`
	Rows    []RowData    `json:"rows"`
}

// UnmarshalJSON handles items where "items" might be a string (empty) or an object
func (i *ItemsData) UnmarshalJSON(data []byte) error {
	// If the items field is an empty object or missing, set empty defaults
	if len(data) == 0 || string(data) == "{}" {
		return nil
	}

	type Alias ItemsData
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(i),
	}
	return json.Unmarshal(data, &aux)
}

// ColumnData represents a column
type ColumnData struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Format string `json:"format"`
}

// RowData represents a row
type RowData struct {
	RecordID string `json:"record_id"`
	Content  ContentMap `json:"content"`
}

// ContentMap handles both array and object content formats
// Array format: [{"type":"text","html":"...","value":"...","column_id":0}]
// Object format: {"0":{"type":"text","html":"...","value":"...","column_id":0}}
type ContentMap map[string]any

// UnmarshalJSON handles both array and object formats
func (c *ContentMap) UnmarshalJSON(data []byte) error {
	*c = make(map[string]any)

	// Try object format first
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err == nil {
		// Check if it looks like our content object
		if _, hasType := obj["type"]; hasType {
			// Single object - use it directly
			*c = obj
			return nil
		}
		// Check if values look like content objects
		for _, v := range obj {
			if mv, ok := v.(map[string]any); ok {
				if _, hasType := mv["type"]; hasType {
					*c = obj
					return nil
				}
			}
		}
	}

	// Try array format
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err == nil {
		// Convert array to map keyed by column_id
		for _, item := range arr {
			if colID, ok := item["column_id"]; ok {
				switch v := colID.(type) {
				case float64:
					key := fmt.Sprintf("%d", int(v))
					(*c)[key] = item
				}
			}
		}
		return nil
	}

	return fmt.Errorf("unknown content format")
}

// ContentValue represents the value within a content field
type ContentValue struct {
	Type       string `json:"type"`
	HTML       string `json:"html"`
	Value      string `json:"value"`
	ColumnID   int    `json:"column_id"`
	ColumnFormat string `json:"column_format"`
}

// ============ Flavor Data Structures ============

// StoreFlavors represents the daily flavors for a single store location
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

// ============ RSS Feed Structures ============

// RSSFeed is the top-level RSS document
type RSSFeed struct {
	XMLName xml.Name  `xml:"rss"`
	Version string    `xml:"version,attr"`
	Channel Channel   `xml:"channel"`
}

// Channel represents the RSS channel
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

// Item represents a single RSS item
type Item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

// ============ Scraper ============

// Scraper handles fetching and parsing the Joe's Italian Ice page
type Scraper struct {
	client  *http.Client
	baseURL string
	timeout time.Duration
}

// NewScraper creates a new Scraper instance
func NewScraper() *Scraper {
	return &Scraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://joesice.com",
		timeout: 30 * time.Second,
	}
}

// FetchPage fetches the main page HTML
func (s *Scraper) FetchPage(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Use browser-like User-Agent headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://joesice.com/")
	// Don't set Accept-Encoding - let the transport handle it via Transport

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}

	return string(body), nil
}

// ExtractEmbeddedTables extracts the embedded tablesomeTables JSON from HTML
func ExtractEmbeddedTables(html string) ([]EmbeddedTable, error) {
	// Pattern to match the window.tablesomeTables = [...] variable
	pattern := regexp.MustCompile(`window\.tablesomeTables\s*=\s*(\[[\s\S]*?\])\s*;`)
	matches := pattern.FindStringSubmatch(html)
	if len(matches) < 2 {
		return nil, fmt.Errorf("could not find tablesomeTables in page")
	}

	var tables []EmbeddedTable
	if err := json.Unmarshal([]byte(matches[1]), &tables); err != nil {
		return nil, fmt.Errorf("failed to parse embedded JSON: %w", err)
	}

	return tables, nil
}

// ParseStoreFlavors parses the embedded tables into structured StoreFlavors
func ParseStoreFlavors(tables []EmbeddedTable) map[int]*StoreFlavors {
	stores := make(map[int]*StoreFlavors)

	// Group tables by location
	// Anaheim: table 20683 (flavors) + 20752 (date/time)
	// Tempe: table 20697 (flavors) + 20754 (date/time)
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

	// Create StoreFlavors for each location
	for tableID, table := range flavorTables {
		store := &StoreFlavors{
			TableID:    tableID,
			ScrapeTime: time.Now(),
		}

		// Extract flavors from rows
		for _, row := range table.Items.Rows {
			if flavor := extractContentValue(row.Content); flavor != "" {
				store.Flavors = append(store.Flavors, flavor)
			}
		}

		// Set store name and location based on table ID
		switch tableID {
		case 20683:
			store.StoreName = "Joe's Italian Ice - Anaheim"
			store.Location = "Anaheim, CA"
			store.OrderURL = "https://order.online/store/joe's-italian-ice-anaheim-2285412/?hideModal=true&pickup=true&redirected=true"
		case 20697:
			store.StoreName = "Joe's Italian Ice - Tempe"
			store.Location = "Tempe, AZ"
			store.OrderURL = "https://order.online/store/joe's-italian-ice-tempe-353993/?delivery=true&hideModal=true&redirected=true"
		}

		// Merge time/date info from corresponding time table
		switch tableID {
		case 20683:
			if timeTable, ok := timeTables[20752]; ok {
				mergeTimeInfo(timeTable, store)
			}
		case 20697:
			if timeTable, ok := timeTables[20754]; ok {
				mergeTimeInfo(timeTable, store)
			}
		}

		// Deduplicate and clean up flavors
		store.Flavors = deduplicateFlavors(store.Flavors)

		stores[tableID] = store
	}

	return stores
}

// mergeTimeInfo extracts date, update time, and store hours from a time table
func mergeTimeInfo(table EmbeddedTable, store *StoreFlavors) {
	// Extract date from column name (e.g., "Saturday May 16, 2026")
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
		value := extractContentValue(row.Content)
		if value == "" {
			continue
		}

		// Check if this looks like an update time
		if strings.HasPrefix(value, "Latest Update:") {
			store.LatestUpdate = value
			continue
		}

		// Check if this looks like store hours
		if strings.HasPrefix(value, "Store Hours:") {
			store.StoreHours = value
			continue
		}
	}
}

// extractContentValue extracts the string value from a ContentMap
func extractContentValue(content ContentMap) string {
	// Try direct access first (object format)
	if val, ok := content["value"].(string); ok {
		return val
	}
	if val, ok := content["html"].(string); ok {
		return val
	}

	// Try keyed access (array format converted to map)
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

// FetchAllFlavors fetches and parses all store flavors from the page
func (s *Scraper) FetchAllFlavors() ([]*StoreFlavors, error) {
	fmt.Println("  Fetching page from Joe's Italian Ice...")
	html, err := s.FetchPage(s.baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}

	fmt.Printf("  Received %d bytes of HTML\n", len(html))

	fmt.Println("  Extracting embedded table data...")
	tables, err := ExtractEmbeddedTables(html)
	if err != nil {
		return nil, fmt.Errorf("failed to extract tables: %w", err)
	}

	fmt.Printf("  Found %d tables\n", len(tables))

	fmt.Println("  Parsing store flavors...")
	storeMap := ParseStoreFlavors(tables)

	var stores []*StoreFlavors
	for _, store := range storeMap {
		stores = append(stores, store)
		fmt.Printf("  ✓ %s: %d flavors\n", store.StoreName, len(store.Flavors))
	}

	return stores, nil
}

// ============ Helpers ============

// deduplicateFlavors removes duplicate flavors while preserving order
func deduplicateFlavors(flavors []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, f := range flavors {
		normalized := strings.TrimSpace(f)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		result = append(result, normalized)
	}
	return result
}

// escapeXML escapes special XML characters
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// ============ RSS Generator ============

// GenerateRSS creates an RSS feed from the store flavors data
func GenerateRSS(stores []*StoreFlavors) *RSSFeed {
	now := time.Now().UTC()

	rss := &RSSFeed{
		Version: "2.0",
		Channel: Channel{
			Title:       "Joe's Italian Ice - Daily Flavors",
			Link:        "https://joesice.com/",
			Description: "Daily ice cream and Italian ice flavors at Joe's locations",
			Language:    "en-us",
			Copyright:   fmt.Sprintf("Copyright %d Joe's Italian Ice", now.Year()),
			Generator:   "joes-italian-ice-rss-feeds v1.0",
			LastBuild:   now.Format(time.RFC1123Z),
			PubDate:     now.Format(time.RFC1123Z),
			TTL:         "60",
		},
	}

	for _, store := range stores {
		item := Item{
			Title:   fmt.Sprintf("Daily Flavors - %s (%s)", store.StoreName, store.Date),
			Link:    store.OrderURL,
			PubDate: store.ScrapeTime.UTC().Format(time.RFC1123Z),
			GUID:    fmt.Sprintf("flavors-%d-%s", store.TableID, store.ScrapeTime.Format("20060102")),
		}

		// Build flavor list description with HTML
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("<h3>%s</h3>", escapeHTML(store.Location)))
		sb.WriteString(fmt.Sprintf("<p><strong>Date:</strong> %s</p>", escapeHTML(store.Date)))
		sb.WriteString(fmt.Sprintf("<p><strong>Latest Update:</strong> %s</p>", escapeHTML(store.LatestUpdate)))
		sb.WriteString(fmt.Sprintf("<p><strong>Store Hours:</strong> %s</p>", escapeHTML(store.StoreHours)))
		sb.WriteString("<p><strong>Available Flavors:</strong></p><ul>")
		for _, flavor := range store.Flavors {
			sb.WriteString(fmt.Sprintf("<li>%s</li>", escapeHTML(flavor)))
		}
		sb.WriteString("</ul>")

		if store.OrderURL != "" {
			sb.WriteString(fmt.Sprintf("<p><a href=\"%s\">Order Online</a></p>", store.OrderURL))
		}

		item.Description = sb.String()
		rss.Channel.Item = append(rss.Channel.Item, item)
	}

	return rss
}

// escapeHTML escapes special HTML characters
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// ============ Main ============

func main() {
	fmt.Println("=== Joe's Italian Ice - Daily Flavors Scraper ===")
	fmt.Println()

	// Parse command line arguments
	var format string
	var output string
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-f", "--format":
			if i+1 < len(os.Args) {
				format = os.Args[i+1]
				i++
			}
		case "-o", "--output":
			if i+1 < len(os.Args) {
				output = os.Args[i+1]
				i++
			}
		case "--serve":
			format = "serve"
		case "--help":
			printUsage()
			return
		default:
			fmt.Printf("Unknown argument: %s\n", os.Args[i])
		}
	}

	if format == "" {
		format = "json"
	}

	// Create scraper and fetch data
	scraper := NewScraper()
	fmt.Println("Fetching daily flavors from Joe's Italian Ice...")
	fmt.Println()

	stores, err := scraper.FetchAllFlavors()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(stores) == 0 {
		fmt.Fprintf(os.Stderr, "No flavors found\n")
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("Successfully fetched %d store(s)\n", len(stores))

	switch format {
	case "json":
		outputJSON(stores, output)
	case "xml", "rss":
		rss := GenerateRSS(stores)
		outputXML(rss, output)
	case "serve":
		startServer(stores)
	case "text":
		outputText(stores)
	default:
		fmt.Fprintf(os.Stderr, "Unknown format: %s\n", format)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: joes-italian-ice-rss-feeds [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -f, --format FORMAT   Output format: json, xml, rss, text, serve (default: json)")
	fmt.Println("  -o, --output FILE     Write to file instead of stdout")
	fmt.Println("  --serve               Start web server to serve feeds")
	fmt.Println("  --help                Show this help message")
}

func outputJSON(stores []*StoreFlavors, output string) {
	data, err := json.MarshalIndent(stores, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	if output != "" {
		if err := os.WriteFile(output, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("JSON written to %s\n", output)
	} else {
		fmt.Println(string(data))
	}
}

func outputXML(rss *RSSFeed, output string) {
	data, err := xml.MarshalIndent(rss, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling XML: %v\n", err)
		os.Exit(1)
	}

	xmlHeader := []byte(xml.Header)
	fullData := append(xmlHeader, data...)

	if output != "" {
		if err := os.WriteFile(output, fullData, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("RSS XML written to %s\n", output)
	} else {
		fmt.Println(string(fullData))
	}
}

func outputText(stores []*StoreFlavors) {
	for _, store := range stores {
		fmt.Printf("=== %s (%s) ===\n", store.StoreName, store.Date)
		fmt.Printf("Update: %s | Hours: %s\n\n", store.LatestUpdate, store.StoreHours)
		for i, flavor := range store.Flavors {
			fmt.Printf("  %2d. %s\n", i+1, flavor)
		}
		fmt.Println()
	}
}

func startServer(stores []*StoreFlavors) {
	// Serve JSON
	http.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data, _ := json.MarshalIndent(stores, "", "  ")
		w.Write(data)
	})

	// Serve RSS feed
	http.HandleFunc("/rss", func(w http.ResponseWriter, r *http.Request) {
		rss := GenerateRSS(stores)
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		data, _ := xml.MarshalIndent(rss, "", "  ")
		w.Write([]byte(xml.Header))
		w.Write(data)
	})

	// Health check
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Joe's Italian Ice RSS Feed Server\n\n")
		fmt.Fprintf(w, "Endpoints:\n")
		fmt.Fprintf(w, "  /rss  - RSS feed\n")
		fmt.Fprintf(w, "  /json - JSON data\n")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("\nStarting server on :%s\n", port)
	fmt.Println("  http://localhost:" + port + "/rss")
	fmt.Println("  http://localhost:" + port + "/json")
	fmt.Println()
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
