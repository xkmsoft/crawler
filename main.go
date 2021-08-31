package main

import (
	"crawler/collector"
	"crawler/searcher"
	"fmt"
	"log"
)

func main() {

	seed := "https://vtk.org/"
	file := "results.json"
	depth := 2

	c, err := collector.NewCollector(seed, depth, true, file)
	if err != nil {
		log.Fatalf("Collector could not be initilaized: %s\n", err.Error())
	}
	_, err = c.StartCrawling()
	if err != nil {
		log.Fatalf("Crawling failed: %s\n", err.Error())
	}

	// Let's use the inverted indexer to apply a full text search over the collector document
	indexer, err := searcher.NewIndexer()
	if err != nil {
		log.Fatalf("Indexer could not be initialized: %s\n", err.Error())
	}
	err = indexer.LoadCollectorDocument(file, true)
	if err != nil {
		log.Fatalf("Loading collector document failed: %s\n", err.Error())
	}

	searchPhrase := "thread count may be especially useful when the processing"
	results := indexer.Search(searchPhrase)
	fmt.Printf("Search results size: %d\n", len(results))
	for _, result := range results {
		fmt.Printf("Rank: %f --> URL: %s\n", result.Rank, result.Url)
	}
}
