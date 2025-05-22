package easyrqst

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	generalPayload   = map[string]any{"name": "morpheus", "age": 30, "email": "example@example.com"}
	multipartPayload = map[string]string{"name": "morpheus", "age": "30", "email": "example@example.com"}
	xmlPayload       = map[string]interface{}{"person": map[string]interface{}{"name": "John Doe", "age": "30", "address": map[string]interface{}{"city": "New York", "state": "NY"}}}
)

// MockCache is a mock implementation of ICacheFn for testing.
type MockCache struct {
	GetFunc    func(key string) (any, error)
	SetFunc    func(key string, value any, expiry time.Duration) (any, error)
	DeleteFunc func(key string) error
	Store      map[string]any
}

func NewMockCache() *MockCache {
	return &MockCache{
		Store: make(map[string]any),
	}
}

func (m *MockCache) Get(key string) (any, error) {
	if m.GetFunc != nil {
		return m.GetFunc(key)
	}
	val, ok := m.Store[key]
	if !ok {
		return nil, fmt.Errorf("key not found in cache") // Simulate cache miss
	}
	return val, nil
}

func (m *MockCache) Set(key string, value any, expiry time.Duration) (any, error) {
	if m.SetFunc != nil {
		return m.SetFunc(key, value, expiry)
	}
	m.Store[key] = value
	return value, nil
}

func (m *MockCache) Delete(key string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(key)
	}
	delete(m.Store, key)
	return nil
}

func TestGetJsonRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok_get"})
	}))
	defer server.Close()

	client := NewHttpClient(server.URL)
	outcome, err := client.Get(WithHeaders(map[string]string{"Content-Type": "application/json"}))

	assert.NoError(t, err)
	assert.NotNil(t, outcome)
	assert.Equal(t, http.StatusOK, outcome.StatusCode)
	assert.Contains(t, string(outcome.Body), "ok_get")
}

func TestPostJsonRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var reqBody map[string]any
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		assert.NoError(t, err)
		assert.Equal(t, generalPayload["name"], reqBody["name"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok_post"})
	}))
	defer server.Close()

	client := NewHttpClient(server.URL)
	outcome, err := client.Post(
		WithPayload(generalPayload),
		WithHeaders(map[string]string{"Content-Type": "application/json"}),
	)

	assert.NoError(t, err)
	assert.NotNil(t, outcome)
	assert.Equal(t, http.StatusCreated, outcome.StatusCode)
	assert.Contains(t, string(outcome.Body), "ok_post")
}

func TestPostMultipartRoute(t *testing.T) {
	// Create a dummy file for upload
	dummyFilePath := filepath.Join(t.TempDir(), "img.png")
	err := os.WriteFile(dummyFilePath, []byte("dummy image data"), 0666)
	assert.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		contentType := r.Header.Get("Content-Type")
		assert.True(t, strings.HasPrefix(contentType, "multipart/form-data"), "Content-Type should be multipart/form-data")

		err := r.ParseMultipartForm(10 << 20) // 10 MB
		assert.NoError(t, err)
		assert.Equal(t, multipartPayload["name"], r.FormValue("name"))

		file, handler, err := r.FormFile("fileKey") // Ensure "fileKey" matches WithFiles
		assert.NoError(t, err)
		assert.NotNil(t, file)
		assert.Equal(t, "img.png", handler.Filename)
		defer file.Close()
		fileBytes, err := io.ReadAll(file)
		assert.NoError(t, err)
		assert.Equal(t, "dummy image data", string(fileBytes))

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok_multipart"})
	}))
	defer server.Close()

	client := NewHttpClient(server.URL)
	outcome, err := client.Post(
		WithPayload(multipartPayload),
		WithHeaders(map[string]string{"Content-Type": "multipart/form-data"}), // This will be overridden by handleMultipartFormData
		WithFiles(map[string]string{"fileKey": dummyFilePath}),
	)

	assert.NoError(t, err)
	assert.NotNil(t, outcome)
	assert.Equal(t, http.StatusCreated, outcome.StatusCode)
	assert.Contains(t, string(outcome.Body), "ok_multipart")
}

func TestPostXmlRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/xml", r.Header.Get("Content-Type"))

		var reqBodyStructure struct {
			XMLName xml.Name `xml:"person"`
			Name    string   `xml:"name"`
			Age     string   `xml:"age"`
			Address struct {
				City  string `xml:"city"`
				State string `xml:"state"`
			} `xml:"address"`
		}
		err := xml.NewDecoder(r.Body).Decode(&reqBodyStructure)
		assert.NoError(t, err)
		assert.Equal(t, "John Doe", reqBodyStructure.Name)

		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("<response><status>ok_xml</status></response>"))
	}))
	defer server.Close()

	client := NewHttpClient(server.URL)
	outcome, err := client.Post(
		WithPayload(xmlPayload),
		WithHeaders(map[string]string{"Content-Type": "application/xml"}),
	)

	assert.NoError(t, err)
	assert.NotNil(t, outcome)
	assert.Equal(t, http.StatusCreated, outcome.StatusCode)
	assert.Contains(t, string(outcome.Body), "ok_xml")
}

func TestPostFormUrlencodedRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		err := r.ParseForm()
		assert.NoError(t, err)
		assert.Equal(t, multipartPayload["name"], r.FormValue("name"))
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("ok_form"))
	}))
	defer server.Close()

	client := NewHttpClient(server.URL)
	outcome, err := client.Post(
		WithPayload(multipartPayload), // multipartPayload is map[string]string, suitable here
		WithHeaders(map[string]string{"Content-Type": "application/x-www-form-urlencoded"}),
	)

	assert.NoError(t, err)
	assert.NotNil(t, outcome)
	assert.Equal(t, http.StatusCreated, outcome.StatusCode)
	assert.Contains(t, string(outcome.Body), "ok_form")
}

// Tests for helper functions
func TestHandleMultipartFormData(t *testing.T) {
	// Create a dummy file
	dummyFilePath := filepath.Join(t.TempDir(), "test.txt")
	err := os.WriteFile(dummyFilePath, []byte("test data"), 0666)
	assert.NoError(t, err)

	payload := map[string]string{"field1": "value1"}
	files := map[string]string{"file1": dummyFilePath}

	buffer, contentType, err := handleMultipartFormData(payload, files)
	assert.NoError(t, err)
	assert.NotNil(t, buffer)
	assert.NotEmpty(t, contentType)
	assert.True(t, strings.HasPrefix(contentType, "multipart/form-data; boundary="))

	// Verify content (simplified)
	body := buffer.String()
	assert.Contains(t, body, `name="field1"`)
	assert.Contains(t, body, `value1`)
	assert.Contains(t, body, `name="file1"`)
	assert.Contains(t, body, `filename="test.txt"`)
	assert.Contains(t, body, "test data")

	// Test file open error
	_, _, err = handleMultipartFormData(payload, map[string]string{"file1": "nonexistent/file.txt"})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrOpenFile)
}

func TestConvertToXMLElementsAndHandleXMLData(t *testing.T) {
	data := map[string]interface{}{
		"root": map[string]interface{}{
			"child1": "value1",
			"child2": map[string]interface{}{
				"grandchild": "value2",
			},
		},
		"anotherRoot": "value3",
	}

	elements := convertToXMLElements(data)
	assert.Len(t, elements, 2)

	// Check root
	var rootEl xmlElement
	for _, el := range elements {
		if el.XMLName.Local == "root" {
			rootEl = el
			break
		}
	}
	assert.NotNil(t, rootEl)
	assert.Len(t, rootEl.Children, 2) // child1, child2

	xmlBytes, err := handleXMLData(data)
	assert.NoError(t, err)
	assert.NotNil(t, xmlBytes)
	xmlString := string(xmlBytes)
	assert.Contains(t, xmlString, "<root>")
	assert.Contains(t, xmlString, "<child1>value1</child1>")
	assert.Contains(t, xmlString, "<grandchild>value2</grandchild>")
	assert.Contains(t, xmlString, "<anotherRoot>value3</anotherRoot>")

	// Test handleXMLData with a type that causes marshal error (not directly possible with current structure)
	// We can test ErrMarshalPayload if convertToXMLElements produced something unmarshalable,
	// but convertToXMLElements itself is quite robust for map[string]interface{}.
}

// Tests for caching
func TestCaching(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "from_server"})
	}))
	defer server.Close()

	mockCache := NewMockCache()
	client := NewHttpClient(server.URL, WithLogger(nil)) // Disable logger for cleaner test output

	// 1. First call - cache miss
	opts := []TReqOption{
		WithHeaders(map[string]string{"Content-Type": "application/json"}),
		WithCache(mockCache, 5*time.Minute, "test_idempotency_key"),
	}
	resp1, err := client.Get(opts...)
	assert.NoError(t, err)
	assert.NotNil(t, resp1)
	assert.Equal(t, http.StatusOK, resp1.StatusCode)
	assert.False(t, resp1.FromCache, "First response should not be from cache")
	assert.Contains(t, string(resp1.Body), "from_server")
	// expectedCacheKey := fmt.Sprintf("%s_%s_%s", http.MethodGet, "test_idempotency_key", server.URL[len("http://"):]) // Simplified key generation for test
	assert.NotEmpty(t, resp1.CacheKey(), "Cache key should be set on response")
	
	// Check if item is in mock cache (actual key format might be more complex, this is a simplified check)
	_, foundInStore := mockCache.Store[resp1.CacheKey()]
	assert.True(t, foundInStore, "Response should be stored in cache")


	// 2. Second call - cache hit
	resp2, err := client.Get(opts...)
	assert.NoError(t, err)
	assert.NotNil(t, resp2)
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	assert.True(t, resp2.FromCache, "Second response should be from cache")
	assert.Equal(t, resp1.Body, resp2.Body, "Cached response body should match original") // Body comparison
	assert.Equal(t, resp1.CacheKey(), resp2.CacheKey())
}

func TestCacheSetError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	mockCache := NewMockCache()
	mockCache.SetFunc = func(key string, value any, expiry time.Duration) (any, error) {
		return nil, fmt.Errorf("forced cache set error")
	}

	client := NewHttpClient(server.URL)
	opts := []TReqOption{WithCache(mockCache, 1*time.Minute, "test")}
	resp, err := client.Get(opts...)

	assert.NoError(t, err) // The Get call itself should not fail due to cache set error
	assert.NotNil(t, resp)
	assert.False(t, resp.FromCache)
	// Check logs (if logger was configured and accessible) or ensure no panic
}

func TestCacheGetError(t *testing.T) {
	serverCallCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCallCount++
		json.NewEncoder(w).Encode(map[string]string{"status": "ok_from_server"})
	}))
	defer server.Close()

	mockCache := NewMockCache()
	mockCache.GetFunc = func(key string) (any, error) {
		return nil, fmt.Errorf("forced cache get error")
	}
	// Pre-populate store to ensure GetFunc is indeed called and its error handled
	mockCache.Store["somekey"] = "somevalue"


	client := NewHttpClient(server.URL)
	opts := []TReqOption{WithCache(mockCache, 1*time.Minute, "test")}
	
	// First call, Get error, should fetch from server
	resp1, err := client.Get(opts...)
	assert.NoError(t, err)
	assert.NotNil(t, resp1)
	assert.False(t, resp1.FromCache)
	assert.Equal(t, 1, serverCallCount, "Server should be called on cache get error")
	assert.Contains(t, string(resp1.Body), "ok_from_server")

	// Override GetFunc to return data that will cause toStruct's json.Marshal to fail.
	mockCache.GetFunc = func(key string) (any, error) {
		// This will cause json.Marshal in toStruct to return an error.
		return make(chan int), nil 
	}
	mockCache.SetFunc = func(key string, value any, expiry time.Duration) (any, error) {
		// successful set after the get error + server fetch
		mockCache.Store[key] = value 
		return value, nil
	}
	
	// Second call, Get returns invalid data, should fetch from server again
	resp2, err := client.Get(opts...)
	assert.NoError(t, err)
	assert.NotNil(t, resp2)
	assert.False(t, resp2.FromCache, "Should not be from cache if conversion failed")
	assert.Equal(t, 2, serverCallCount, "Server should be called again if cached data is invalid")
	assert.Contains(t, string(resp2.Body), "ok_from_server")
}


// Tests for prepareRequest error paths
func TestPrepareRequestError_InvalidPayloadType(t *testing.T) {
	client := NewHttpClient("http://dummyurl.com") // URL doesn't matter as prepareRequest fails earlier

	// Test application/x-www-form-urlencoded with wrong payload type
	_, err := client.Post(
		WithPayload(123), // Not map[string]string
		WithHeaders(map[string]string{"Content-Type": "application/x-www-form-urlencoded"}),
	)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidPayloadTypeForm, "Error should be ErrInvalidPayloadTypeForm")

	// Test multipart/form-data with wrong payload type
	_, err = client.Post(
		WithPayload(123), // Not map[string]string
		WithHeaders(map[string]string{"Content-Type": "multipart/form-data"}),
	)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidPayloadTypeMulti, "Error should be ErrInvalidPayloadTypeMulti")

	// Test application/xml with wrong payload type
	_, err = client.Post(
		WithPayload("not a map"), // Not map[string]interface{}
		WithHeaders(map[string]string{"Content-Type": "application/xml"}),
	)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidPayloadTypeXML, "Error should be ErrInvalidPayloadTypeXML")

	// Test default (json) with unmarshallable payload
	_, err = client.Post(
		WithPayload(make(chan int)), // Cannot be marshalled to JSON
		WithHeaders(map[string]string{"Content-Type": "application/json"}),
	)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrMarshalPayload, "Error should be ErrMarshalPayload for unmarshallable JSON")
}

func TestPrepareRequestError_HandleMultipartError(t *testing.T) {
	client := NewHttpClient("http://dummyurl.com")
	// Test error from handleMultipartFormData (e.g., file not found)
	_, err := client.Post(
		WithPayload(map[string]string{"field": "value"}),
		WithHeaders(map[string]string{"Content-Type": "multipart/form-data"}),
		WithFiles(map[string]string{"aFile": "nonexistent/path/to/file.txt"}),
	)
	assert.Error(t, err)
	// The error from handleMultipartFormData is wrapped, so check for underlying ErrOpenFile
	assert.ErrorContains(t, err, ErrOpenFile.Error())
}

func TestCustomRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "custom_ok"})
	}))
	defer server.Close()

	client := NewHttpClient(server.URL)
	outcome, err := client.Custom(http.MethodPatch, WithPayload(map[string]string{"data":"payload"}))
	assert.NoError(t, err)
	assert.NotNil(t, outcome)
	assert.Equal(t, http.StatusOK, outcome.StatusCode)
	assert.Contains(t, string(outcome.Body), "custom_ok")
}

func TestDefaultContentType(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // For POST/PUT/PATCH etc. Content-Type is expected.
        // For GET, Content-Type header on request is not standard but can be tested if server expects it.
        // This test focuses on POST where it's more relevant.
        if r.Method == http.MethodPost {
            assert.Equal(t, "application/json", r.Header.Get("Content-Type"), "Default Content-Type should be application/json")
        }
        w.WriteHeader(http.StatusOK)
    }))
    defer server.Close()

    client := NewHttpClient(server.URL)
    
    // Test POST without explicit Content-Type header
    _, err := client.Post(WithPayload(map[string]string{"key": "value"}))
    assert.NoError(t, err)

    // Test GET (Content-Type not typically sent by client, but ensure no error)
    _, err = client.Get()
    assert.NoError(t, err)
}
