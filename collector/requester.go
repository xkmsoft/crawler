package collector

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 11_5_2) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/92.0.4515.159 Safari/537.36"
)

type Requester interface {
	HeadRequest(url string) (*http.Response, error)
	GetRequest(url string) (*http.Response, error)
	Request(url string, method string) (*http.Response, error)
}

type Request struct {
	UserAgent  string
	Client     *http.Client
	Timeout    time.Duration
}

func NewRequest(timeout time.Duration) *Request {
	return &Request{
		UserAgent: userAgent,
		Client:    &http.Client{Timeout: timeout},
		Timeout:   timeout,
	}
}

func (r *Request) HeadRequest(url string) (*http.Response, error)  {
	return r.Request(url, "HEAD")
}

func (r *Request) GetRequest(url string) (*http.Response, error) {
	return r.Request(url, "GET")
}

func (r *Request) Request(url string, method string) (*http.Response, error) {
	request, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("new http request failed: %s\n", err.Error()))
	}
	request.Header.Set("User-Agent", r.UserAgent)

	response, err := r.Client.Do(request)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("http request failed: %s\n", err.Error()))
	}
	if response.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("status code: %d\n", response.StatusCode))
	}
	return response, nil
}
