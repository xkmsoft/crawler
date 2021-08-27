package collector

import (
	"errors"
	"github.com/microcosm-cc/bluemonday"
	"net/url"
	"regexp"
	"strings"
	"time"
)

func AbsoluteURL(pageURL string, href string) (string, error) {
	if strings.HasPrefix(href, "#") {
		return "", errors.New("url has #")
	}
	baseURL, err := url.ParseRequestURI(pageURL)
	if err != nil {
		return "", errors.New("page url could not be parsed")
	}
	absURL, err := baseURL.Parse(href)
	if err != nil {
		return "", errors.New("base url could not be parsed with href")
	}
	absURL.Fragment = ""
	if absURL.Scheme == "//" {
		absURL.Scheme = baseURL.Scheme
	}
	if absURL.Scheme != "http" && absURL.Scheme != "https" {
		return "", errors.New("unknown scheme")
	}
	return absURL.String(), nil
}

func TrimAndSanitize(s string) string {
	p := bluemonday.StrictPolicy()
	s = p.Sanitize(s)
	s = strings.TrimSpace(s)
	// Removing spaces
	re := regexp.MustCompile(`[\s\p{Zs}]{2,}`)
	s = re.ReplaceAllString(s, "")
	return s
}

func URLExists(urls []string, url string) bool {
	for _, item := range urls {
		if item == url {
			return true
		}
	}
	return false
}

func CurrentTimestamp() int64 {
	return time.Now().UTC().Unix()
}
