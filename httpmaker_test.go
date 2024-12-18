package easyrqst

import (
	"fmt"
	"testing"
	"time"
)

const endpoint = "http://localhost:9000"

var (
	generalPayload   = map[string]any{"name": "morpheus", "age": 30, "email": "example@example.com"}
	multipartPayload = map[string]string{"name": "morpheus", "age": "30", "email": "example@example.com"}
	complexPayload   = map[string]interface{}{"person": map[string]interface{}{"name": "John Doe", "age": "30", "address": map[string]interface{}{"city": "New York", "state": "NY"}}}
)

func TestGetJsonRoute(t *testing.T) {
	call := NewHttpClient(fmt.Sprintf("%s/json", endpoint), WithRetry(4), WithRetryWaitMax(time.Millisecond*100))
	headers := WithHeaders(map[string]string{"Content-Type": "application/json"})

	outcome, err := call.Get(headers)
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}
	if outcome.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %v", outcome.StatusCode)
	}
	t.Log(string(outcome.Body))
}

func TestPostJsonRoute(t *testing.T) {
	call := NewHttpClient(fmt.Sprintf("%s/json", endpoint), WithRetry(4), WithRetryWaitMax(time.Millisecond*100))

	body := WithPayload(generalPayload)
	headers := WithHeaders(map[string]string{"Content-Type": "application/json"})

	outcome, err := call.Post(body, headers)
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}
	if outcome.StatusCode != 200 {
		t.Errorf("Expected status code 201, got %v", outcome.StatusCode)
	}
	t.Log(string(outcome.Body))
}

func TestPostMultipartRoute(t *testing.T) {
	call := NewHttpClient(fmt.Sprintf("%s/multipart", endpoint), WithRetry(4), WithRetryWaitMax(time.Millisecond*100))

	body := WithPayload(multipartPayload)
	headers := WithHeaders(map[string]string{"Content-Type": "multipart/form-data"})
	files := WithFiles(map[string]string{"files": "example/img.png"})
	outcome, err := call.Post(body, headers, files)
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}
	if outcome.StatusCode != 200 {
		t.Errorf("Expected status code 201, got %v", outcome.StatusCode)
	}
	t.Log(string(outcome.Body))

}

func TestGetXmlRoute(t *testing.T) {
	call := NewHttpClient(fmt.Sprintf("%s/xml", endpoint), WithRetry(4), WithRetryWaitMax(time.Millisecond*100))

	headers := WithHeaders(map[string]string{"Content-Type": "application/xml"})

	outcome, err := call.Get(headers)
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}
	if outcome.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %v", outcome.StatusCode)
	}
	t.Log(string(outcome.Body))
}

func TestPostXmlRoute(t *testing.T) {
	call := NewHttpClient(fmt.Sprintf("%s/xml", endpoint), WithRetry(4), WithRetryWaitMax(time.Millisecond*100))

	body := WithPayload(complexPayload)
	headers := WithHeaders(map[string]string{"Content-Type": "application/xml"})

	outcome, err := call.Post(body, headers)
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}
	if outcome.StatusCode != 200 {
		t.Errorf("Expected status code 201, got %v", outcome.StatusCode)
	}
	t.Log(string(outcome.Body))
}

func TestGetFormUrlencodedRoute(t *testing.T) {
	call := NewHttpClient(fmt.Sprintf("%s/form", endpoint), WithRetry(4), WithRetryWaitMax(time.Millisecond*100))

	headers := WithHeaders(map[string]string{"Content-Type": "application/x-www-form-urlencoded"})

	outcome, err := call.Get(headers)
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}
	if outcome.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %v", outcome.StatusCode)
	}
	t.Log(string(outcome.Body))
}

func TestPostFormUrlencodedRoute(t *testing.T) {
	call := NewHttpClient(fmt.Sprintf("%s/form", endpoint), WithRetry(4), WithRetryWaitMax(time.Millisecond*100))

	body := WithPayload(multipartPayload)
	headers := WithHeaders(map[string]string{"Content-Type": "application/x-www-form-urlencoded"})

	outcome, err := call.Post(body, headers)
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}
	if outcome.StatusCode != 200 {
		t.Errorf("Expected status code 201, got %v", outcome.StatusCode)
	}
	t.Log(string(outcome.Body))
}
