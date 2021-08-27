package collector

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type SucceededPage struct {
	Url           string   `json:"url"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	ContentType   string   `json:"content_type"`
	ContentLength int64    `json:"content_length"`
	Timestamp     int64    `json:"timestamp"`
	Urls          []string `json:"urls"`
	Paragrahps    []string `json:"paragrahps"`
}

type FailedPage struct {
	Url        string `json:"url"`
	FailReason string `json:"fail_reason"`
	Timestamp  int64  `json:"timestamp"`
}

type ScrapeResult struct {
	Page  *SucceededPage
	Error error
}

type ScraperInterface interface {
	Id() int
	InitiateScrape(url string)
	ScrapeSucceed(url string, page *SucceededPage)
	ScrapeFailed(url string, page *FailedPage)
	IsProcessed(url string) bool
	IsVisited(url string) bool
	IsFailed(url string) bool
	NumberOfPagesSucceed() int
	NumberOfPagesFailed() int
	NumberOfPagesBeingProcessed() int
	Scrape(url string, channel chan ScrapeResult, wg *sync.WaitGroup)
}

type Scrapper struct {
	Succeed   map[string]*SucceededPage `json:"succeed"`
	Failed    map[string]*FailedPage    `json:"failed"`
	InProcess map[string]int
	Loggers   *Loggers
	Mutex     sync.Mutex
}

func NewScrapper(loggers *Loggers) *Scrapper {
	return &Scrapper{
		Succeed:   map[string]*SucceededPage{},
		Failed:    map[string]*FailedPage{},
		InProcess: map[string]int{},
		Loggers:   loggers,
		Mutex:     sync.Mutex{},
	}
}

func (s *Scrapper) Id() int {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, _ := strconv.Atoi(idField)
	return id
}

func (s *Scrapper) InitiateScrape(url string) {
	s.Mutex.Lock()
	s.InProcess[url] = s.Id()
	s.Mutex.Unlock()
}

func (s *Scrapper) ScrapeSucceed(url string, page *SucceededPage) {
	s.Mutex.Lock()
	s.Succeed[url] = page
	delete(s.InProcess, url)
	s.Loggers.Log(INFO, fmt.Sprintf("Scrape succeded on page :%s\n", page.Url))
	s.Mutex.Unlock()
}

func (s *Scrapper) ScrapeFailed(url string, page *FailedPage) {
	s.Mutex.Lock()
	s.Failed[url] = page
	delete(s.InProcess, url)
	s.Loggers.Log(INFO, fmt.Sprintf("Scrape failed on page: %s Reason: %s\n", page.Url, page.FailReason))
	s.Mutex.Unlock()
}

func (s *Scrapper) IsProcessed(url string) bool {
	s.Mutex.Lock()
	_, ok := s.InProcess[url]
	defer s.Mutex.Unlock()
	if ok {
		return false
	}
	return true
}

func (s *Scrapper) IsVisited(url string) bool {
	s.Mutex.Lock()
	_, ok := s.Succeed[url]
	defer s.Mutex.Unlock()
	if ok {
		return true
	}
	return false
}

func (s *Scrapper) IsFailed(url string) bool {
	s.Mutex.Lock()
	_, ok := s.Failed[url]
	defer s.Mutex.Unlock()
	if ok {
		return true
	}
	return false
}

func (s *Scrapper) NumberOfPagesSucceed() int {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	return len(s.Succeed)
}

func (s *Scrapper) NumberOfPagesFailed() int {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	return len(s.Failed)
}

func (s *Scrapper) NumberOfPagesBeingProcessed() int {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	return len(s.InProcess)
}

func (s *Scrapper) Scrape(url string, channel chan ScrapeResult, wg *sync.WaitGroup) {
	defer wg.Done()

	if url == "" {
		channel <- ScrapeResult{Page: nil, Error: errors.New("empty url")}
		return
	}

	if s.IsVisited(url) {
		channel <- ScrapeResult{Page: nil, Error: errors.New("page already visited")}
		return
	}

	if !s.IsProcessed(url) {
		channel <- ScrapeResult{Page: nil, Error: errors.New("page still being processed")}
		return
	}

	s.InitiateScrape(url)

	requester := NewRequest(30 * time.Second)

	headResponse, headError := requester.HeadRequest(url)
	if headError != nil {
		s.ScrapeFailed(url, &FailedPage{Url: url, FailReason: headError.Error(), Timestamp: CurrentTimestamp()})
		channel <- ScrapeResult{Page: nil, Error: headError}
		return
	}
	defer headResponse.Body.Close()

	contentLength := headResponse.ContentLength
	contentType := strings.ToLower(headResponse.Header.Get("Content-Type"))

	if !strings.Contains(contentType, "text/html") {
		page := &SucceededPage{
			Url:           url,
			Title:         "",
			ContentType:   contentType,
			ContentLength: contentLength,
			Description:   "",
			Timestamp:     CurrentTimestamp(),
			Urls:          []string{},
			Paragrahps:    []string{},
		}
		s.ScrapeSucceed(url, page)
		channel <- ScrapeResult{Page: page, Error: nil}
		return
	}

	getResponse, getError := requester.GetRequest(url)
	if getError != nil {
		s.ScrapeFailed(url, &FailedPage{Url: url, FailReason: getError.Error(), Timestamp: CurrentTimestamp()})
		channel <- ScrapeResult{Page: nil, Error: getError}
		return
	}
	defer getResponse.Body.Close()

	var title, description string
	var urls = []string{}
	var paragraphs = []string{}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(getResponse.Body)
	if err != nil {
		s.ScrapeFailed(url, &FailedPage{Url: url, FailReason: err.Error(), Timestamp: CurrentTimestamp()})
		channel <- ScrapeResult{Page: nil, Error: err}
		return
	}

	// Find page title
	doc.Find("title").Each(func(i int, s *goquery.Selection) {
		title = TrimAndSanitize(s.Text())
	})

	// Find page meta-data
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		if name, _ := s.Attr("name"); strings.EqualFold(name, "description") {
			// proceed to extract and use the content of description
			content, exists := s.Attr("content")
			if exists {
				description = content
			} else {
				description = title
			}
		}
	})

	// Find page paragraphs
	doc.Find("p").Each(func(i int, s *goquery.Selection) {
		para := TrimAndSanitize(s.Text())
		if para != "" {
			paragraphs = append(paragraphs, para)
		}
	})

	// Find the urls within the page
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			absoluteUrl, err := AbsoluteURL(url, href)
			if err != nil {
				return
			}
			if !URLExists(urls, absoluteUrl) {
				urls = append(urls, absoluteUrl)
			}
		}
	})

	page := &SucceededPage{
		Url:           url,
		Title:         title,
		ContentType:   contentType,
		ContentLength: contentLength,
		Description:   description,
		Timestamp:     CurrentTimestamp(),
		Urls:          urls,
		Paragrahps:    paragraphs,
	}
	s.ScrapeSucceed(url, page)
	channel <- ScrapeResult{Page: page, Error: nil}
}
