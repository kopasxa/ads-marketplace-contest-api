package statsparser

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"go.uber.org/zap"
)

type PostStat struct {
	MessageID int64     `json:"message_id"`
	URL       string    `json:"url"`
	Date      time.Time `json:"date"`
	Views     *int      `json:"views,omitempty"`
	TextSnippet string  `json:"text_snippet,omitempty"`
}

type ChannelStats struct {
	Username      string     `json:"username"`
	Subscribers   *int       `json:"subscribers,omitempty"`
	VerifiedBadge bool       `json:"verified_badge"`
	LastPosts     []PostStat `json:"last_posts"`
	AvgViewsLast20 *int      `json:"avg_views_last_20,omitempty"`
	LangGuess     string     `json:"lang_guess"`
	FetchedAt     time.Time  `json:"fetched_at"`
}

type Parser struct {
	httpClient *http.Client
	log        *zap.Logger
	timeout    time.Duration
	maxRetries int
}

func NewParser(timeoutMS, maxRetries int, log *zap.Logger) *Parser {
	return &Parser{
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutMS) * time.Millisecond,
		},
		log:        log,
		timeout:    time.Duration(timeoutMS) * time.Millisecond,
		maxRetries: maxRetries,
	}
}

func (p *Parser) FetchAndParse(ctx context.Context, username string) (*ChannelStats, error) {
	url := fmt.Sprintf("https://t.me/s/%s", username)

	var doc *goquery.Document
	var lastErr error

	for attempt := 0; attempt <= p.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")

		resp, err := p.httpClient.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
			continue
		}

		doc, err = goquery.NewDocumentFromReader(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		lastErr = nil
		break
	}

	if lastErr != nil {
		return nil, lastErr
	}

	stats := &ChannelStats{
		Username:  username,
		FetchedAt: time.Now(),
	}

	// Parse subscribers
	doc.Find(".tgme_channel_info_counter .counter_value").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		parent := s.Parent()
		label := strings.ToLower(strings.TrimSpace(parent.Find(".counter_type").Text()))
		if strings.Contains(label, "subscriber") || strings.Contains(label, "member") {
			n := parseCount(text)
			if n > 0 {
				stats.Subscribers = &n
			}
		}
	})

	// Fallback: try tgme_channel_info_header_counter
	if stats.Subscribers == nil {
		doc.Find(".tgme_channel_info_header_counter").Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if strings.Contains(strings.ToLower(text), "subscriber") || strings.Contains(strings.ToLower(text), "member") {
				n := parseCount(text)
				if n > 0 {
					stats.Subscribers = &n
				}
			}
		})
	}

	// Verified badge
	stats.VerifiedBadge = doc.Find(".tgme_channel_info_header_title .verified-icon").Length() > 0 ||
		doc.Find(".tgme_channel_info_header_title i.verified-icon").Length() > 0

	// Parse posts
	var allText strings.Builder
	doc.Find(".tgme_widget_message_wrap").Each(func(i int, s *goquery.Selection) {
		post := PostStat{}

		// Message ID from data-post
		if dataPost, exists := s.Find(".tgme_widget_message").Attr("data-post"); exists {
			parts := strings.Split(dataPost, "/")
			if len(parts) == 2 {
				if id, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					post.MessageID = id
					post.URL = fmt.Sprintf("https://t.me/%s/%d", username, id)
				}
			}
		}

		// Date
		s.Find(".tgme_widget_message_date time").Each(func(_ int, timeEl *goquery.Selection) {
			if dt, exists := timeEl.Attr("datetime"); exists {
				if t, err := time.Parse(time.RFC3339, dt); err == nil {
					post.Date = t
				}
			}
		})

		// Views
		s.Find(".tgme_widget_message_views").Each(func(_ int, viewEl *goquery.Selection) {
			text := strings.TrimSpace(viewEl.Text())
			n := parseCount(text)
			if n > 0 {
				post.Views = &n
			}
		})

		// Text snippet
		text := strings.TrimSpace(s.Find(".tgme_widget_message_text").Text())
		if len(text) > 200 {
			text = text[:200]
		}
		post.TextSnippet = text
		allText.WriteString(text)
		allText.WriteString(" ")

		if post.MessageID > 0 {
			stats.LastPosts = append(stats.LastPosts, post)
		}
	})

	// Avg views last 20
	if len(stats.LastPosts) > 0 {
		total := 0
		count := 0
		for i, p := range stats.LastPosts {
			if i >= 20 {
				break
			}
			if p.Views != nil {
				total += *p.Views
				count++
			}
		}
		if count > 0 {
			avg := total / count
			stats.AvgViewsLast20 = &avg
		}
	}

	// Last post ID
	if len(stats.LastPosts) > 0 {
		// The last post is the most recent
	}

	// Language guess
	stats.LangGuess = guessLanguage(allText.String())

	return stats, nil
}

// FetchPostContent fetches a specific post page and returns its text content + hash.
func (p *Parser) FetchPostContent(ctx context.Context, username string, messageID int64) (string, bool, error) {
	url := fmt.Sprintf("https://t.me/%s/%d?embed=1", username, messageID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", false, nil // post deleted
	}
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", false, err
	}

	text := strings.TrimSpace(doc.Find(".tgme_widget_message_text").Text())
	if text == "" {
		// Might be a media-only post, check if the message widget exists
		if doc.Find(".tgme_widget_message").Length() == 0 {
			return "", false, nil // deleted
		}
	}

	return text, true, nil
}

var viewCountRE = regexp.MustCompile(`[\d,.]+[KkMm]?`)

func parseCount(text string) int {
	text = strings.ReplaceAll(text, " ", "")
	text = strings.ReplaceAll(text, ",", "")

	match := viewCountRE.FindString(text)
	if match == "" {
		return 0
	}

	multiplier := 1
	if strings.HasSuffix(match, "K") || strings.HasSuffix(match, "k") {
		multiplier = 1000
		match = match[:len(match)-1]
	} else if strings.HasSuffix(match, "M") || strings.HasSuffix(match, "m") {
		multiplier = 1000000
		match = match[:len(match)-1]
	}

	f, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0
	}
	return int(f * float64(multiplier))
}

func guessLanguage(text string) string {
	if text == "" {
		return "unknown"
	}

	cyrillicCount := 0
	latinCount := 0
	arabicCount := 0
	cjkCount := 0
	totalLetters := 0

	for _, r := range text {
		if !unicode.IsLetter(r) {
			continue
		}
		totalLetters++
		if unicode.Is(unicode.Cyrillic, r) {
			cyrillicCount++
		} else if unicode.Is(unicode.Latin, r) {
			latinCount++
		} else if unicode.Is(unicode.Arabic, r) {
			arabicCount++
		} else if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			cjkCount++
		}
	}

	if totalLetters == 0 {
		return "unknown"
	}

	cyrPct := float64(cyrillicCount) / float64(totalLetters)
	latPct := float64(latinCount) / float64(totalLetters)
	arPct := float64(arabicCount) / float64(totalLetters)
	cjkPct := float64(cjkCount) / float64(totalLetters)

	switch {
	case cyrPct >= 0.3:
		return "ru" // could be uk/bg etc, but MVP
	case arPct >= 0.3:
		return "ar"
	case cjkPct >= 0.3:
		return "zh"
	case latPct >= 0.3:
		return "en" // simplified, could be any latin language
	default:
		return "other"
	}
}
