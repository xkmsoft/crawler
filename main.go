package main

import (
	"crawler/collector"
	"log"
)

func main() {
	seed := "https://vtk.org/"
	depth := 2

	c, err := collector.NewCollector(seed, depth, true, "vtk_org.json")
	if err != nil {
		log.Fatalf("Collector could not be initilaized: %s\n", err.Error())
	}
	_, err = c.StartCrawling()
	if err != nil {
		log.Fatalf("Crawling failed: %s\n", err.Error())
	}
}
