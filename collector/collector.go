package collector

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"sync"
	"time"
)

const (
	LogFile = "logs.txt"
)

const (
	INFO    = iota
	WARNING = iota
	ERROR   = iota
)

type Crawler interface {
	StartCrawling() (int, error)
	Crawl(page *SucceededPage, depth int)
	SaveResultsToFile() (bool, error)
}

type LoggersInterface interface {
	Log(t int, msg string)
}

type Loggers struct {
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger
	Mutex   sync.Mutex
}

func (l *Loggers) Log(t int, msg string) {
	l.Mutex.Lock()
	switch t {
	case INFO:
		l.Info.Printf(msg)
	case WARNING:
		l.Warning.Printf(msg)
	case ERROR:
		l.Error.Printf(msg)
	}
	l.Mutex.Unlock()
}

func CreateLoggers(fileName string) (*Loggers, error) {
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return nil, err
	}
	infoLogger := log.New(file, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	warningLogger := log.New(file, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
	errorLogger := log.New(file, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	return &Loggers{
		Info:    infoLogger,
		Warning: warningLogger,
		Error:   errorLogger,
	}, nil
}

type Collector struct {
	Seed       string
	Depth      int
	SaveToFile bool
	FileName   string
	Scrapper   *Scrapper
	Loggers    *Loggers
	Begin      time.Time
	End        time.Time
}

type ResultData struct {
	Seed               string                    `json:"seed"`
	Depth              int                       `json:"depth"`
	BeginTimestamp     time.Time                 `json:"begin_timestamp"`
	EndTimestamp       time.Time                 `json:"end_timestamp"`
	ExecutionInSeconds float64                   `json:"execution_in_seconds"`
	PageRatePerSec     float64                   `json:"page_rate_per_sec"`
	TotalPages         int                       `json:"total_pages"`
	SucceededPages     int                       `json:"succeeded_pages"`
	FailedPages        int                       `json:"failed_pages"`
	Succeed            map[string]*SucceededPage `json:"succeed"`
	Failed             map[string]*FailedPage    `json:"failed"`
}

func NewCollector(seed string, depth int, saveToFile bool, fileName string) (*Collector, error) {
	_, err := url.ParseRequestURI(seed)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("seed is not valid url: %s", err.Error()))
	}
	if depth <= 0 {
		return nil, errors.New("depth should be a positive number")
	}
	loggers, err := CreateLoggers(LogFile)
	if err != nil {
		fmt.Printf("Error creating loggers: %s\n", err.Error())
	}
	c := &Collector{
		Seed:       seed,
		Depth:      depth,
		SaveToFile: saveToFile,
		FileName:   fileName,
		Scrapper:   NewScrapper(loggers),
		Loggers:    loggers,
	}
	return c, nil
}

func (c *Collector) StartCrawling() (int, error) {
	message := fmt.Sprintf("Crawling starting for url: %s with depth: %d\n", c.Seed, c.Depth)
	fmt.Printf(message)
	c.Loggers.Log(INFO, message)
	c.Begin = time.Now()
	var wg sync.WaitGroup
	channel := make(chan ScrapeResult)
	wg.Add(1)
	go c.Scrapper.Scrape(c.Seed, channel, &wg)

	scrapeResult := <-channel
	if scrapeResult.Error != nil {
		c.Loggers.Log(ERROR, fmt.Sprintf("Scrape error: %s\n", scrapeResult.Error.Error()))
	}
	if scrapeResult.Page != nil {
		c.Crawl(scrapeResult.Page, c.Depth-1)
	}
	wg.Wait()
	if c.SaveToFile {
		c.End = time.Now()
		_, _ = c.SaveResultsToFile()
	}
	return c.Scrapper.NumberOfPagesSucceed(), nil
}

func (c *Collector) Crawl(page *SucceededPage, depth int) {
	if page == nil {
		return
	}
	if depth <= 0 {
		return
	}
	var wg sync.WaitGroup
	wg.Add(len(page.Urls))
	channel := make(chan ScrapeResult)
	for _, u := range page.Urls {
		go c.Scrapper.Scrape(u, channel, &wg)
	}
	for range page.Urls {
		scrapeResult := <-channel
		if scrapeResult.Error != nil {
			c.Loggers.Log(ERROR, fmt.Sprintf("Scrape error: %s\n", scrapeResult.Error.Error()))
		}
		if scrapeResult.Page != nil {
			c.Crawl(scrapeResult.Page, depth-1)
		}
	}
	wg.Wait()
	return
}

func (c *Collector) SaveResultsToFile() (bool, error) {
	c.Loggers.Log(INFO, fmt.Sprintf("Collecting finished %d pages scrapped successfully %d pages failed\n",
		c.Scrapper.NumberOfPagesSucceed(),
		c.Scrapper.NumberOfPagesFailed(),
	))
	executionInSec := time.Since(c.Begin).Seconds()
	succeededPages := c.Scrapper.NumberOfPagesSucceed()
	failedPages := c.Scrapper.NumberOfPagesFailed()
	totalPages := succeededPages + failedPages
	pageRatePerSec := float64(totalPages) / executionInSec
	data := &ResultData{
		Seed:               c.Seed,
		Depth:              c.Depth,
		BeginTimestamp:     c.Begin,
		EndTimestamp:       c.End,
		ExecutionInSeconds: executionInSec,
		PageRatePerSec:     pageRatePerSec,
		TotalPages:         totalPages,
		SucceededPages:     succeededPages,
		FailedPages:        failedPages,
		Succeed:            c.Scrapper.Succeed,
		Failed:             c.Scrapper.Failed,
	}
	file, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		c.Loggers.Log(ERROR, fmt.Sprintf("Error marshalling to json the results: %s\n", err.Error()))
		return false, err
	}
	err = ioutil.WriteFile(c.FileName, file, 0644)
	if err != nil {
		c.Loggers.Log(ERROR, fmt.Sprintf("Error saving the results into the file: %s\n", err.Error()))
		return false, err
	}
	c.Loggers.Log(INFO, fmt.Sprintf("Results saved successfully into the file :%s\n", c.FileName))
	return true, nil
}
