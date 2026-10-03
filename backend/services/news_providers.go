package services

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type NewsProvider interface {
	Name() string
	Fetch(context.Context) ([]Article, error)
}

// Operator-configured feeds still cannot reach private networks or redirect keys.
func PublicProviderURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return publicIP(ip)
	}
	return true
}
func publicIP(ip net.IP) bool {
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified()
}
func providerClient() *http.Client {
	transport := &http.Transport{TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 10 * time.Second, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(ips) == 0 {
			return nil, errors.New("provider DNS unavailable")
		}
		for _, ip := range ips {
			if !publicIP(ip.IP) {
				return nil, errors.New("provider must use public addresses")
			}
		}
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}}
	return &http.Client{Timeout: 12 * time.Second, Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
}
func providerRead(ctx context.Context, client *http.Client, raw string) ([]byte, error) {
	if !PublicProviderURL(raw) {
		return nil, errors.New("provider URL must be public HTTPS")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, errors.New("invalid provider URL")
	}
	req.Header.Set("User-Agent", "PROPHIT-News/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("provider request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("provider returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (4<<20)+1))
	if err != nil || len(data) > 4<<20 {
		return nil, errors.New("invalid or oversized provider response")
	}
	return data, nil
}

type GNewsProvider struct {
	Client       *http.Client
	BaseURL, Key string
	Categories   []string
	PageSize     int
}

func (p GNewsProvider) Name() string { return "gnews" }
func (p GNewsProvider) Fetch(ctx context.Context) ([]Article, error) {
	if p.Key == "" {
		return nil, ErrNewsNotConfigured
	}
	all := []Article{}
	for _, category := range p.Categories {
		u, err := url.Parse(p.BaseURL)
		if err != nil {
			return nil, errors.New("invalid GNews URL")
		}
		q := u.Query()
		q.Set("apikey", p.Key)
		q.Set("category", category)
		q.Set("lang", "en")
		q.Set("max", strconv.Itoa(p.PageSize))
		q.Set("from", time.Now().UTC().Add(-24*time.Hour).Format(time.RFC3339))
		u.RawQuery = q.Encode()
		data, err := providerRead(ctx, p.Client, u.String())
		if err != nil {
			return nil, err
		}
		var response GNewsResponse
		if json.Unmarshal(data, &response) != nil {
			return nil, errors.New("invalid GNews response")
		}
		for _, a := range response.Articles {
			a.Provider = p.Name()
			a.Category = newsCategory(category, a.Title+" "+a.Description)
			if a.Language == "" {
				a.Language = "en"
			}
			all = append(all, a)
		}
	}
	return all, nil
}

type rssItem struct {
	ID          string `xml:"guid"`
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	Published   string `xml:"pubDate"`
}
type atomItem struct {
	ID        string `xml:"id"`
	Title     string `xml:"title"`
	Summary   string `xml:"summary"`
	Published string `xml:"published"`
	Links     []struct {
		Href string `xml:"href,attr"`
		Rel  string `xml:"rel,attr"`
	} `xml:"link"`
}
type RSSProvider struct {
	Client *http.Client
	Feeds  []string
}

func (p RSSProvider) Name() string { return "rss" }
func (p RSSProvider) Fetch(ctx context.Context) ([]Article, error) {
	all := []Article{}
	failures := 0
	for _, feed := range p.Feeds {
		data, err := providerRead(ctx, p.Client, feed)
		if err != nil {
			failures++
			continue
		}
		var doc struct {
			Channel struct {
				Title string    `xml:"title"`
				Items []rssItem `xml:"item"`
			} `xml:"channel"`
			Title   string     `xml:"title"`
			Entries []atomItem `xml:"entry"`
		}
		if xml.Unmarshal(data, &doc) != nil {
			failures++
			continue
		}
		for _, item := range doc.Channel.Items {
			date, err := parseNewsDate(item.Published)
			if err != nil {
				continue
			}
			all = append(all, Article{ID: item.ID, Title: item.Title, Description: item.Description, URL: item.Link, PublishedAt: date.Format(time.RFC3339), Source: ArticleSource{Name: doc.Channel.Title}, Provider: p.Name(), Language: "en", Category: newsCategory("", item.Title+" "+item.Description)})
		}
		for _, item := range doc.Entries {
			date, err := parseNewsDate(item.Published)
			if err != nil {
				continue
			}
			link := ""
			for _, l := range item.Links {
				if l.Rel == "" || l.Rel == "alternate" {
					link = l.Href
					break
				}
			}
			all = append(all, Article{ID: item.ID, Title: item.Title, Description: item.Summary, URL: link, PublishedAt: date.Format(time.RFC3339), Source: ArticleSource{Name: doc.Title}, Provider: p.Name(), Language: "en", Category: newsCategory("", item.Title+" "+item.Summary)})
		}
	}
	if len(all) == 0 && failures > 0 {
		return nil, errors.New("RSS feeds unavailable")
	}
	return all, nil
}
func parseNewsDate(raw string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822} {
		if date, err := time.Parse(layout, raw); err == nil {
			return date.UTC(), nil
		}
	}
	return time.Time{}, errors.New("invalid publication date")
}
func newsCategory(category, text string) string {
	if _, ok := CategoryPayouts[category]; ok {
		return category
	}
	text = strings.ToLower(text)
	switch {
	case category == "sports":
		return "Sports"
	case category == "entertainment":
		return "Entertainment"
	case category == "business":
		return "Financial Markets"
	case strings.Contains(text, "weather") || strings.Contains(text, "rainfall") || strings.Contains(text, "temperature"):
		return "Weather"
	case strings.Contains(text, "election") || strings.Contains(text, "parliament"):
		return "Politics"
	default:
		return "Wild Card"
	}
}
func envDefault(key, value string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return value
}
func envInt(key string, value, min, max int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil || v < min || v > max {
		return value
	}
	return v
}
func NewsRefreshInterval() time.Duration {
	return time.Duration(envInt("NEWS_REFRESH_MINUTES", 60, 15, 1440)) * time.Minute
}
func ConfiguredNewsProviders() []NewsProvider {
	providers := []NewsProvider{}
	client := providerClient()
	if key := os.Getenv("NEWS_API_KEY"); key != "" {
		categories := strings.Split(envDefault("NEWS_CATEGORIES", "general,sports,business,entertainment"), ",")
		if len(categories) > 4 {
			categories = categories[:4]
		}
		providers = append(providers, GNewsProvider{Client: client, BaseURL: envDefault("NEWS_PROVIDER_URL", "https://gnews.io/api/v4/top-headlines"), Key: key, Categories: categories, PageSize: envInt("NEWS_PAGE_SIZE", 50, 1, 100)})
	}
	if raw := strings.TrimSpace(os.Getenv("NEWS_RSS_FEEDS")); raw != "" {
		feeds := strings.Split(raw, ",")
		if len(feeds) > 4 {
			feeds = feeds[:4]
		}
		for i := range feeds {
			feeds[i] = strings.TrimSpace(feeds[i])
		}
		providers = append(providers, RSSProvider{Client: client, Feeds: feeds})
	}
	return providers
}
func fetchProviders(ctx context.Context, providers []NewsProvider) ([]Article, string, error) {
	if len(providers) == 0 {
		return nil, "", ErrNewsNotConfigured
	}
	for _, provider := range providers {
		articles, err := provider.Fetch(ctx)
		if err == nil && len(articles) > 0 {
			return articles, provider.Name(), nil
		}
	}
	return nil, "", errors.New("all configured news providers unavailable or empty")
}
