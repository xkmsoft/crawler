package searcher

import (
	"crawler/collector"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"time"
)

type SearchResult struct {
	Url  string  `json:"url"`
	Rank float64 `json:"rank"`
}

type WikiXMLDoc struct {
	Title string `xml:"title"`
	Url string `xml:"url"`
	Abstract string `xml:"abstract"`
}

type WikiXMLDump struct {
	Documents []WikiXMLDoc `xml:"doc"`
}

type IndexerInterface interface {
	LoadCollectorDocument(path string) error
	LoadWikimediaDump(path string) error
	LoadIndexDump(path string) error
	SaveIndexDump() error
	Analyze(s string) []string
	AddIndex(tokens []string, url string)
	Search(s string) []SearchResult
	FindMax(frequency map[string]int) int
}

type Indexer struct {
	Indexes   map[string][]string
	Tokenizer *Tokenizer
	Filterer  *Filterer
	Stemmer   *Stemmer
}

func NewIndexer() (*Indexer, error) {
	filterer, err := NewFilterer()
	if err != nil {
		return nil, err
	}
	return &Indexer{
		Indexes:   map[string][]string{},
		Tokenizer: NewTokenizer(),
		Filterer:  filterer,
		Stemmer:   NewStemmer(),
	}, nil
}

func (i *Indexer) LoadCollectorDocument(path string, save bool) error {
	jsonFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func(jsonFile *os.File) {
		err := jsonFile.Close()
		if err != nil {
			fmt.Printf("Error closing json file: %s\n", err.Error())
		}
	}(jsonFile)

	bytes, _ := ioutil.ReadAll(jsonFile)

	var resultData collector.ResultData

	err = json.Unmarshal(bytes, &resultData)
	if err != nil {
		return err
	}
	for url, page := range resultData.Succeed {
		// Page title
		i.AddIndex(i.Analyze(page.Title), url)
		// Page Description
		i.AddIndex(i.Analyze(page.Description), url)
		// Page paragraphs
		for _, paragraph := range page.Paragrahps {
			i.AddIndex(i.Analyze(paragraph), url)
		}
	}
	if save {
		err := i.SaveIndexDump()
		if err != nil {
			fmt.Printf("Saving indexes dump failed: %s\n", err.Error())
		}
	}
	return nil
}

func (i *Indexer) LoadWikimediaDump(path string, save bool) error {
	begin := time.Now()
	defer func(begin time.Time) {
		elapsed := time.Since(begin)
		fmt.Printf("Loading wikimedia dump took %f seconds\n", elapsed.Seconds())
	}(begin)
	xmlFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func(xmlFile *os.File) {
		err := xmlFile.Close()
		if err != nil {
			fmt.Printf("Closing xml file failed: %s\n", err.Error())
		}
	}(xmlFile)

	var dump WikiXMLDump
	decoder := xml.NewDecoder(xmlFile)

	if err := decoder.Decode(&dump); err != nil {
		return err
	}

	// TODO: Implement to process documents concurrently, since it takes a lot of time to index the wiki dump

	fmt.Printf("Number of documents: %d\n", len(dump.Documents))

	for idx, doc := range dump.Documents {
		i.AddIndex(i.Analyze(doc.Title), doc.Url)
		i.AddIndex(i.Analyze(doc.Abstract), doc.Url)
		if idx % 1000 == 0 {
			fmt.Printf("%dk documents are indexed\n", idx / 1000)
		}
	}

	if save {
		err := i.SaveIndexDump()
		if err != nil {
			fmt.Printf("Saving indexes dump failed: %s\n", err.Error())
		}
	}
	return nil
}

func (i *Indexer) LoadIndexDump(path string) error {
	begin := time.Now()
	defer func(begin time.Time) {
		elapsed := time.Since(begin)
		fmt.Printf("Loading indexes dump took %f seconds\n", elapsed.Seconds())
	}(begin)

	jsonFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func(jsonFile *os.File) {
		err := jsonFile.Close()
		if err != nil {
			fmt.Printf("Error closing json file: %s\n", err.Error())
		}
	}(jsonFile)

	bytes, _ := ioutil.ReadAll(jsonFile)

	var indexes map[string][]string

	err = json.Unmarshal(bytes, &indexes)
	if err != nil {
		return err
	}
	i.Indexes = indexes
	return nil
}

func (i *Indexer) SaveIndexDump() error {
	file, err := json.MarshalIndent(i.Indexes, "", "  ")
	if err != nil {
		fmt.Printf("Error marshalling to json the results: %s\n", err.Error())
		return err
	}
	err = ioutil.WriteFile("indexes.json", file, 0644)
	if err != nil {
		fmt.Printf("Error saving the indexes dump into the file: %s\n", err.Error())
		return err
	}
	fmt.Printf("Indexes dump saved successfully into the file\n")
	return nil
}

func (i *Indexer) Analyze(s string) []string {
	tokens := i.Tokenizer.Tokenize(s)
	tokens = i.Filterer.Lowercase(tokens)
	tokens = i.Filterer.RemoveStopWords(tokens)
	tokens = i.Stemmer.Stem(tokens)
	return tokens
}

func (i *Indexer) AddIndex(tokens []string, url string) {
	for _, token := range tokens {
		urls, exists := i.Indexes[token]
		if exists {
			if !collector.URLExists(urls, url) {
				urls = append(urls, url)
			}
			i.Indexes[token] = urls
		} else {
			i.Indexes[token] = []string{url}
		}
	}
}

func (i *Indexer) Search(s string) []SearchResult {
	begin := time.Now()
	defer func(begin time.Time, phrase string) {
		elapsed := time.Since(begin)
		fmt.Printf("Search took %d micro seconds for phrase: %s\n", elapsed.Microseconds(), phrase)
	}(begin, s)

	results := []SearchResult{}
	frequency := map[string]int{}
	tokens := i.Analyze(s)
	for _, token := range tokens {
		urls, exists := i.Indexes[token]
		if exists {
			for _, url := range urls {
				v, ok := frequency[url]
				if ok {
					frequency[url] = v + 1
				} else {
					frequency[url] = 1
				}
			}
		}
	}
	max := i.FindMax(frequency)
	for url, freq := range frequency {
		rank := float64(freq) / float64(max)
		results = append(results, SearchResult{
			Url:  url,
			Rank: rank,
		})
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Rank > results[j].Rank
	})
	return results
}

func (i *Indexer) FindMax(frequency map[string]int) int {
	max := 0
	for _, freq := range frequency {
		if freq > max {
			max = freq
		}
	}
	return max
}
