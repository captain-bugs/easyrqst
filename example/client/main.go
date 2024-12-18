package main

import (
	"encoding/json"
	"errors"
	"github.com/captain-bugs/easyrqst"
	"github.com/patrickmn/go-cache"
	"log"
	"time"
)

type ILocalCache interface {
	Get(key string) (any, error)
	Set(key string, value any, expiry time.Duration) (any, error)
	Delete(key string) error
}

type LocalCache struct {
	cache *cache.Cache
}

func NewLocalCache() ILocalCache {
	return &LocalCache{cache.New(5*time.Minute, 10*time.Minute)}
}

func (l *LocalCache) Get(key string) (any, error) {
	data, found := l.cache.Get(key)
	if !found {
		return nil, errors.New("key not found")
	}
	return data, nil
}

func (l *LocalCache) Set(key string, value any, expiry time.Duration) (any, error) {
	l.cache.Set(key, value, expiry)
	return nil, nil
}

func (l *LocalCache) Delete(key string) error {
	l.cache.Delete(key)
	return nil
}

func GET_Json() {
	log.Println("GET_Json")
	cache := NewLocalCache()

	const endpoint = "http://localhost:9000/json"
	call := easyrqst.NewHttpClient(endpoint, easyrqst.WithRetry(4), easyrqst.WithRetryWaitMax(time.Millisecond*100))
	headers := easyrqst.WithHeaders(map[string]string{"Content-Type": "application/json"})
	caching := easyrqst.WithCache(cache, time.Minute*5, "json")
	outcome, err := call.Get(headers, caching)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	if outcome.StatusCode != 200 {
		log.Fatalf("Expected status code 200, got %v", outcome.StatusCode)
	}
	var data = make(map[string]interface{})
	if err := json.Unmarshal(outcome.Body, &data); err != nil {
		log.Fatalf("Error: %v", err)
	}
	x, _ := cache.Get(outcome.CacheKey())
	log.Println("cached value", x)
	log.Println(data)
}

func GET_UrlEncoded() {
	log.Println("GET_UrlEncoded")
	const endpoint = "http://localhost:9000/form"
	call := easyrqst.NewHttpClient(endpoint, easyrqst.WithRetry(4), easyrqst.WithRetryWaitMax(time.Millisecond*100))
	headers := easyrqst.WithHeaders(map[string]string{"Content-Type": "application/x-www-form-urlencoded"})

	outcome, err := call.Get(headers)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	if outcome.StatusCode != 200 {
		log.Fatalf("Expected status code 200, got %v", outcome.StatusCode)
	}
	log.Println(outcome.Body)
}

func GET_Xml() {
	log.Println("GET_Xml")
	const endpoint = "http://localhost:9000/xml"
	call := easyrqst.NewHttpClient(endpoint, easyrqst.WithRetry(4), easyrqst.WithRetryWaitMax(time.Millisecond*100))
	headers := easyrqst.WithHeaders(map[string]string{"Content-Type": "application/xml"})

	outcome, err := call.Get(headers)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	if outcome.StatusCode != 200 {
		log.Fatalf("Expected status code 200, got %v", outcome.StatusCode)
	}
	log.Println(outcome.Body)
}

func POST_Json() {
	log.Println("POST_Json")
	const endpoint = "http://localhost:9000/json"
	call := easyrqst.NewHttpClient(endpoint, easyrqst.WithRetry(4), easyrqst.WithRetryWaitMax(time.Millisecond*100))

	body := easyrqst.WithPayload(map[string]any{"name": "morpheus", "age": 30, "email": "example@example.com"})
	headers := easyrqst.WithHeaders(map[string]string{"Content-Type": "application/json"})

	outcome, err := call.Post(body, headers)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	var data = make(map[string]interface{})
	if err := json.Unmarshal(outcome.Body, &data); err != nil {
		log.Fatalf("Error: %v", err)
	}
	log.Println(data)
}

func POST_Multipart() {
	log.Println("POST_Multipart")
	const endpoint = "http://localhost:9000/multipart"
	call := easyrqst.NewHttpClient(endpoint, easyrqst.WithRetry(4), easyrqst.WithRetryWaitMax(time.Millisecond*100))

	body := easyrqst.WithPayload(map[string]string{"name": "morpheus", "age": "30", "email": "example@example.com"})
	headers := easyrqst.WithHeaders(map[string]string{"Content-Type": "multipart/form-data"})
	files := easyrqst.WithFiles(map[string]string{"files": "../img.png"})

	outcome, err := call.Post(body, headers, files)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	log.Println(outcome.Body)
}

func POST_Xml() {
	log.Println("POST_Xml")
	const endpoint = "http://localhost:9000/xml"
	call := easyrqst.NewHttpClient(endpoint, easyrqst.WithRetry(4), easyrqst.WithRetryWaitMax(time.Millisecond*100))

	body := easyrqst.WithPayload(map[string]interface{}{"person": map[string]interface{}{"name": "John Doe", "age": "30", "address": map[string]interface{}{"city": "New York", "state": "NY"}}})
	headers := easyrqst.WithHeaders(map[string]string{"Content-Type": "application/xml"})

	outcome, err := call.Post(body, headers)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	log.Println(outcome.Body)
}

func POST_UrlEncoded() {
	log.Println("POST_UrlEncoded")
	const endpoint = "http://localhost:9000/form"
	call := easyrqst.NewHttpClient(endpoint, easyrqst.WithRetry(4), easyrqst.WithRetryWaitMax(time.Millisecond*100))

	body := easyrqst.WithPayload(map[string]string{"name": "morpheus", "age": "30", "email": "example@example.com"})
	headers := easyrqst.WithHeaders(map[string]string{"Content-Type": "application/x-www-form-urlencoded"})

	outcome, err := call.Post(body, headers)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	log.Println(outcome.Body)
}

func main() {
	GET_Json()
	GET_UrlEncoded()
	GET_Xml()
	POST_Json()
	POST_Multipart()
	POST_Xml()
	POST_UrlEncoded()
}
