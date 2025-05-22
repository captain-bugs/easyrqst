package easyrqst

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-retryablehttp" // For retryable HTTP requests.

	"errors"
)

// Custom error types define standard errors that can be returned by the package.
// This allows consumers to check for specific error types using errors.Is().
var (
	// ErrInvalidPayload indicates that the provided payload was not of the expected type or format.
	ErrInvalidPayload = errors.New("invalid payload")
	// ErrCacheGet indicates a failure while attempting to retrieve an item from the cache.
	ErrCacheGet = errors.New("failed to get data from cache")
	// ErrCacheSet indicates a failure while attempting to store an item in the cache.
	ErrCacheSet = errors.New("failed to set data to cache")
	// ErrOpenFile indicates a failure when trying to open a file (e.g., for multipart uploads).
	ErrOpenFile = errors.New("failed to open file")
	// ErrWriteField indicates a failure when writing a field to a multipart form.
	ErrWriteField = errors.New("failed to write form field")
	// ErrCreateFormFile indicates a failure when creating a part for a file in a multipart form.
	ErrCreateFormFile = errors.New("failed to create form file")
	// ErrCopyFile indicates a failure when copying file content to a multipart form part.
	ErrCopyFile = errors.New("failed to copy file content")
	// ErrCloseWriter indicates a failure when closing a writer (e.g., a multipart writer).
	ErrCloseWriter = errors.New("failed to close writer")
	// ErrHttpRequestCreate indicates a failure during the creation of an *http.Request.
	ErrHttpRequestCreate = errors.New("failed to create HTTP request")
	// ErrHttpRequestDo indicates a failure when executing an HTTP request using the http.Client.
	ErrHttpRequestDo = errors.New("failed to execute HTTP request")
	// ErrReadResponseBody indicates a failure when reading the body of an *http.Response.
	ErrReadResponseBody = errors.New("failed to read response body")
	// ErrMarshalPayload indicates a failure when marshalling a payload (e.g., to JSON or XML).
	ErrMarshalPayload = errors.New("failed to marshal payload")
	// ErrInvalidPayloadTypeForm indicates that the payload type was incorrect for "application/x-www-form-urlencoded".
	ErrInvalidPayloadTypeForm = errors.New("payload should be a map[string]string for x-www-form-urlencoded")
	// ErrInvalidPayloadTypeMulti indicates that the payload type was incorrect for "multipart/form-data".
	// Specifically, the non-file payload part should be map[string]string.
	ErrInvalidPayloadTypeMulti = errors.New("payload should be a map[string]string for multipart/form-data")
	// ErrInvalidPayloadTypeXML indicates that the payload type was incorrect for "application/xml".
	ErrInvalidPayloadTypeXML = errors.New("payload should be a map[string]interface{} for application/xml")
)

// xmlElement is a helper struct for converting map[string]interface{} to a structure
// that can be marshalled into XML. This allows for dynamic XML tag generation based on map keys.
type xmlElement struct {
	XMLName  xml.Name     // Represents the XML element tag name.
	Attr     []xml.Attr   // Attributes of the XML element (currently not used but available for future extension).
	Content  string       `xml:",chardata"` // Character data content of the element.
	Children []xmlElement `xml:",any"`      // Nested XML elements, allowing for recursive structures.
}

// ICacheFn defines the interface for a cache implementation.
// This allows users of the easyrqst package to provide their own caching mechanisms,
// such as Redis, an in-memory cache, or any other key-value store.
type ICacheFn interface {
	// Get retrieves an item from the cache based on its key.
	// It returns the cached item (as any) and an error if the item is not found or if another error occurs.
	Get(key string) (any, error)
	// Set adds an item to the cache with a specified key and an expiration duration.
	// It returns the stored item (or a confirmation) and an error if the operation fails.
	Set(key string, value any, expiry time.Duration) (any, error)
	// Delete removes an item from the cache by its key.
	// It returns an error if the deletion fails.
	Delete(key string) error
}

// IHttpClient defines the primary interface for the HTTP client provided by easyrqst.
// It offers methods for common HTTP operations like GET and POST,
// as well as a flexible Custom method for other HTTP verbs.
type IHttpClient interface {
	// Get performs an HTTP GET request to the configured endpoint.
	// It accepts a variable number of TReqOption functions to customize the request
	// (e.g., adding headers, query parameters, or configuring caching).
	// Returns an *HttpResponse or an error if the request fails.
	Get(opts ...TReqOption) (*HttpResponse, error)
	// Post performs an HTTP POST request to the configured endpoint.
	// Similar to Get, it accepts TReqOption functions for request customization,
	// particularly for setting the request payload and content type.
	// Returns an *HttpResponse or an error.
	Post(opts ...TReqOption) (*HttpResponse, error)
	// Custom performs an HTTP request using a specified HTTP method (e.g., PUT, DELETE, PATCH).
	// It takes the method string (e.g., http.MethodPut) and TReqOption functions.
	// Returns an *HttpResponse or an error.
	Custom(method string, opts ...TReqOption) (*HttpResponse, error)
}

// TReqOption is a functional option type for configuring request-specific parameters.
// Functions of this type are passed to Get, Post, or Custom methods to modify
// aspects of a single HTTP request, such as its headers, query parameters, payload, or caching settings.
type TReqOption func(*ReqOptions)

// THttpOption is a functional option type for configuring client-wide parameters of the easyRequest instance.
// Functions of this type are passed to NewHttpClient to set properties that affect
// all requests made by that client, such as retry policies or logging.
type THttpOption func(*easyRequest)

// cacheObj holds cache-related settings that can be applied to a request.
// It's typically configured via the WithCache request option.
type cacheObj struct {
	fncs        ICacheFn      // The cache implementation (e.g., Redis, in-memory) that adheres to ICacheFn.
	expiry      time.Duration // The duration for which the response should be cached.
	idempotency string        // An idempotency key or similar unique identifier for the request, used in generating the cache key.
}

// ReqOptions aggregates all configurable options for an individual HTTP request.
// An instance of ReqOptions is created and modified by TReqOption functions for each request.
type ReqOptions struct {
	queries  map[string]string // URL query parameters (key-value pairs).
	headers  map[string]string // HTTP request headers (key-value pairs).
	files    map[string]string // Files to be uploaded in a multipart/form-data request. The map key is the form field name, and the value is the local file path.
	cacheObj *cacheObj         // Cache configuration specific to this request. If nil, caching is disabled or client defaults are used.
	payload  any               // The request body (payload). Its type should be compatible with the specified Content-Type header.
}

// easyRequest is the internal struct that implements the IHttpClient interface.
// It holds the configuration and state for the HTTP client.
type easyRequest struct {
	forceCache   bool          // (Currently unused) A field intended for future use to forcibly bypass or use the cache.
	cacheObj     *cacheObj     // Default cache configuration for all requests made by this client. Can be overridden by per-request cache settings in ReqOptions.
	endpoint     string        // The base URL to which requests are made (e.g., "https://api.example.com").
	client       *http.Client  // The underlying standard Go *http.Client used for making requests. This is typically a client from go-retryablehttp.
	maxRetry     int           // Maximum number of retry attempts for idempotent requests if they fail.
	retryWaitMax time.Duration // Maximum duration to wait between retry attempts.
	logger       interface{}   // A logger instance, compatible with retryablehttp.Logger or retryablehttp.LeveledLogger, for logging retry attempts and profiling.
}

// HttpResponse encapsulates the details of a response received from an HTTP request.
type HttpResponse struct {
	method     string // The HTTP method (e.g., "GET", "POST") of the request that generated this response.
	cacheKey   string // The cache key used for this response, if caching was active. Empty if not cached.
	FromCache  bool   // True if this response was served from the cache, false if it came from the origin server.
	StatusCode int    // The HTTP status code (e.g., 200, 404, 500).
	Body       []byte // The raw byte slice of the HTTP response body.
}

// handleMultipartFormData constructs the body for a "multipart/form-data" request.
// It takes a payload of regular form fields (map[string]string) and a map of files to be uploaded.
// In the files map, the key is the form field name for the file, and the value is its local path.
// Returns a *bytes.Buffer containing the constructed multipart body, the generated "Content-Type"
// string (which includes the unique multipart boundary), and an error if any part of the process fails.
func handleMultipartFormData(payload map[string]string, files map[string]string) (*bytes.Buffer, string, error) {
	var bodyBuffer bytes.Buffer
	multipartWriter := multipart.NewWriter(&bodyBuffer)

	// Write simple form fields to the multipart body.
	for key, val := range payload {
		if err := multipartWriter.WriteField(key, val); err != nil {
			return nil, "", fmt.Errorf("%w: writing field '%s': %v", ErrWriteField, key, err)
		}
	}

	// Write files to the multipart body.
	for fieldName, filePath := range files {
		file, err := os.Open(filePath)
		if err != nil {
			return nil, "", fmt.Errorf("%w: opening file '%s' for field '%s': %v", ErrOpenFile, filePath, fieldName, err)
		}
		// file.Close() is deferred until the end of this function, ensuring it's closed even on errors.
		// However, for optimal resource management in loops, consider closing earlier if possible,
		// though here it's acceptable as the file handle is needed until io.Copy completes.
		defer file.Close()

		partWriter, err := multipartWriter.CreateFormFile(fieldName, filepath.Base(filePath))
		if err != nil {
			return nil, "", fmt.Errorf("%w: creating form file for field '%s': %v", ErrCreateFormFile, fieldName, err)
		}

		if _, err = io.Copy(partWriter, file); err != nil {
			return nil, "", fmt.Errorf("%w: copying file '%s' for field '%s': %v", ErrCopyFile, filePath, fieldName, err)
		}
	}

	// Close the multipart writer to finalize the body. This writes the terminating boundary.
	if err := multipartWriter.Close(); err != nil {
		return nil, "", fmt.Errorf("%w: closing multipart writer: %v", ErrCloseWriter, err)
	}

	return &bodyBuffer, multipartWriter.FormDataContentType(), nil
}

// convertToXMLElements recursively transforms a map[string]interface{} into a slice of xmlElement.
// This is a helper function used by handleXMLData to prepare a data structure suitable for
// marshalling into XML, where map keys become XML element names.
func convertToXMLElements(data map[string]interface{}) []xmlElement {
	var elements []xmlElement
	// Iterate over the input map. Each key-value pair becomes an xmlElement.
	for key, value := range data {
		element := xmlElement{
			XMLName: xml.Name{Local: key}, // Use map key as XML tag name.
		}
		// Handle value based on its type.
		switch v := value.(type) {
		case string:
			element.Content = v // Simple string content.
		case map[string]interface{}:
			element.Children = convertToXMLElements(v) // Recursively convert nested maps.
		default:
			element.Content = fmt.Sprintf("%v", v) // Convert other types to string.
		}
		elements = append(elements, element)
	}
	return elements
}

// handleXMLData prepares and marshals a map[string]interface{} into an XML byte slice.
// It uses convertToXMLElements to create a structure that xml.MarshalIndent can process.
// The resulting XML is indented for readability.
func handleXMLData(data map[string]interface{}) ([]byte, error) {
	xmlBytes, err := xml.MarshalIndent(convertToXMLElements(data), "", "  ")
	if err != nil {
		// Wrap the error from xml.MarshalIndent with additional context.
		return nil, fmt.Errorf("%w: marshalling to XML: %v", ErrMarshalPayload, err)
	}
	return xmlBytes, nil
}

// toStruct is a generic function that converts data from type M (e.g., a map from JSON unmarshalling)
// to a specific struct type S. It achieves this by first marshalling M to JSON,
// and then unmarshalling that JSON into an instance of S.
// This is particularly useful for converting cached data (often stored as map[string]interface{})
// back into the expected *HttpResponse struct or other concrete types.
// Returns the converted struct S or an error if marshalling or unmarshalling fails.
func toStruct[M any, S any](sourceData M) (S, error) {
	var resultStruct S // Initialize an empty struct of type S.

	// Marshal the source data (M) into a temporary JSON byte slice.
	jsonData, err := json.Marshal(sourceData)
	if err != nil {
		return resultStruct, fmt.Errorf("failed to marshal source data for struct conversion: %w", err)
	}

	// Unmarshal the temporary JSON byte slice into the target struct (S).
	if err := json.Unmarshal(jsonData, &resultStruct); err != nil {
		return resultStruct, fmt.Errorf("failed to unmarshal JSON into target struct: %w", err)
	}
	return resultStruct, nil
}

// WithQueries is a request option (TReqOption) used to set URL query parameters for an HTTP request.
// It takes a map where keys are parameter names and values are parameter values.
// Example: client.Get(WithQueries(map[string]string{"id": "123", "status": "active"}))
func WithQueries(queries map[string]string) TReqOption {
	return func(o *ReqOptions) { o.queries = queries }
}

// WithHeaders is a request option (TReqOption) used to set HTTP request headers.
// It takes a map where keys are header names and values are header values.
// Example: client.Post(WithHeaders(map[string]string{"Authorization": "Bearer YOUR_TOKEN", "X-Custom-Header": "value"}))
func WithHeaders(headers map[string]string) TReqOption {
	return func(o *ReqOptions) { o.headers = headers }
}

// WithPayload is a request option (TReqOption) used to set the body (payload) of an HTTP request.
// The payload's type should be compatible with the request's "Content-Type" header.
// For JSON, this is typically a struct or map[string]any. For XML, map[string]interface{}.
// For "application/x-www-form-urlencoded", it should be map[string]string.
// Example: client.Post(WithPayload(myRequestDataStruct{Name: "Test", Value: 10}))
func WithPayload(payload any) TReqOption {
	return func(o *ReqOptions) { o.payload = payload }
}

// WithFiles is a request option (TReqOption) used to specify files for upload in a "multipart/form-data" request.
// It takes a map where keys are the form field names for the files, and values are the local paths to these files.
// This option should be used in conjunction with WithPayload for other form fields if any.
// Example: client.Post(WithPayload(map[string]string{"description":"A test file"}), WithFiles(map[string]string{"upload": "/path/to/your/file.txt"}))
func WithFiles(files map[string]string) TReqOption {
	return func(o *ReqOptions) { o.files = files }
}

// WithCache is a request option (TReqOption) that enables and configures caching for a specific HTTP request.
// - cache: An instance of a cache implementation that satisfies the ICacheFn interface.
// - period: The duration for which the response to this request should be cached.
// - idempotency: A string (e.g., an idempotency key or a unique request signature) used to make the cache key unique and avoid collisions.
// Example: client.Get(WithCache(myRedisCache, 10*time.Minute, "get-user-profile-123"))
func WithCache(cache ICacheFn, period time.Duration, idempotency string) TReqOption {
	return func(o *ReqOptions) {
		o.cacheObj = &cacheObj{fncs: cache, expiry: period, idempotency: idempotency}
	}
}

// WithRetry is a client option (THttpOption) used to set the maximum number of retry attempts for failed requests.
// This configuration applies to all requests made by the client instance created with this option.
// Retries are typically only performed for idempotent methods (GET, PUT, DELETE, etc.) or if the server indicates it's safe.
func WithRetry(max int) THttpOption {
	return func(o *easyRequest) { o.maxRetry = max }
}

// WithRetryWaitMax is a client option (THttpOption) used to set the maximum duration to wait between retry attempts.
// The actual wait time may vary (e.g., exponential backoff) but will not exceed this maximum.
// This applies to all requests made by the client instance.
func WithRetryWaitMax(wait time.Duration) THttpOption {
	return func(o *easyRequest) { o.retryWaitMax = wait }
}

// WithLogger is a client option (THttpOption) used to set a custom logger for the HTTP client.
// The logger should be compatible with `retryablehttp.Logger` (for basic logging) or
// `retryablehttp.LeveledLogger` (for structured, leveled logging).
// This logger is used by the underlying `retryablehttp` client for logging retry attempts and by `easyRequest` for profiling.
func WithLogger(logger interface{}) THttpOption {
	return func(o *easyRequest) { o.logger = logger }
}

// NewHttpClient creates and configures a new HTTP client that implements the IHttpClient interface.
// - endpoint: The base URL for all requests made by this client (e.g., "https://api.example.com/v1").
// - opts: A variadic list of THttpOption functions to configure client-wide settings such as retry policy,
//   custom logger, etc.
// It returns an IHttpClient instance ready to make HTTP requests.
func NewHttpClient(endpoint string, opts ...THttpOption) IHttpClient {
	// Initialize the underlying retryable HTTP client from hashicorp/go-retryablehttp.
	retryClient := retryablehttp.NewClient()

	// Initialize our easyRequest wrapper with default values.
	easyRqstClient := &easyRequest{
		endpoint:     endpoint,
		client:       retryClient.StandardClient(), // Get the stdlib-compatible *http.Client from retryablehttp.
		maxRetry:     3,                            // Default maximum number of retries for requests.
		retryWaitMax: 1 * time.Second,              // Default maximum wait time between retries.
		logger:       nil,                          // By default, no logger is set (retryablehttp might use a discard logger).
	}

	// Apply any client-wide functional options (THttpOption) provided by the caller.
	// These options can override the default settings of easyRqstClient.
	for _, opt := range opts {
		opt(easyRqstClient)
	}

	// Configure the retryablehttp client with the (potentially overridden) settings from easyRqstClient.
	retryClient.RetryMax = easyRqstClient.maxRetry
	retryClient.RetryWaitMax = easyRqstClient.retryWaitMax
	retryClient.Logger = easyRqstClient.logger // Assign the configured logger to the retryablehttp client.

	return easyRqstClient
}

// profile is an internal helper method used for request performance profiling.
// It records the start time when called and returns a function. This returned function,
// when executed (typically via `defer`), calculates the elapsed time since `start`
// and logs it using the configured logger.
func (h *easyRequest) profile(urlPath, method string) func() {
	startTime := time.Now() // Record the start time of the operation.
	return func() { // This closure is returned and usually called with defer.
		elapsedDuration := time.Since(startTime).String() // Calculate the time elapsed since startTime.
		// Log the request duration. It supports both LeveledLogger and standard Logger interfaces.
		switch logger := h.logger.(type) {
		case retryablehttp.LeveledLogger:
			logger.Debug("REQUEST_TIME", "url", urlPath, "method", method, "elapsed", elapsedDuration)
		case retryablehttp.Logger:
			logger.Printf("REQUEST_TIME url=%s method=%s elapsed=%v", urlPath, method, elapsedDuration)
		}
	}
}

// prepareRequest creates and configures an *http.Request object based on the provided method,
// endpoint, and request-specific options (TReqOption).
// This internal method is responsible for:
// - Initializing and applying all request options (like headers, queries, payload, cache settings).
// - Serializing the request payload to the appropriate format (JSON, XML, form-urlencoded)
//   based on the "Content-Type" header.
// - Setting up file uploads for "multipart/form-data" requests.
// It returns a fully prepared *http.Request or an error if any step fails.
func (h *easyRequest) prepareRequest(method, endpointURL string, opts ...TReqOption) (*http.Request, error) {
	// Initialize ReqOptions with default empty maps for queries and headers.
	options := ReqOptions{
		queries: make(map[string]string),
		headers: make(map[string]string),
	}

	// Apply all provided request-specific functional options (TReqOption) to the options struct.
	for _, opt := range opts {
		opt(&options)
	}

	var requestBodyReader io.Reader // Use io.Reader for the request body, allowing various sources.

	// Process payload and files, preparing the requestBodyReader and setting Content-Type if necessary.
	if options.payload != nil || options.files != nil {
		// Determine Content-Type; default later if not explicitly set.
		contentType := options.headers["Content-Type"]

		switch contentType {
		case "application/x-www-form-urlencoded":
			formData, ok := options.payload.(map[string]string)
			if !ok {
				return nil, ErrInvalidPayloadTypeForm // Payload must be map[string]string for this Content-Type.
			}
			urlValues := url.Values{}
			for k, v := range formData {
				urlValues.Set(k, v)
			}
			requestBodyReader = strings.NewReader(urlValues.Encode()) // Use strings.NewReader for efficiency with string.

		case "multipart/form-data":
			var payloadMap map[string]string
			if options.payload != nil { // If payload for non-file fields is provided
				var ok bool
				payloadMap, ok = options.payload.(map[string]string)
				if !ok {
					return nil, ErrInvalidPayloadTypeMulti // Non-file payload part must be map[string]string.
				}
			} else {
				payloadMap = make(map[string]string) // Ensure payloadMap is not nil if only files are sent.
			}

			buffer, generatedContentType, err := handleMultipartFormData(payloadMap, options.files)
			if err != nil {
				return nil, fmt.Errorf("failed to handle multipart form data: %w", err)
			}
			requestBodyReader = buffer
			options.headers["Content-Type"] = generatedContentType // Update Content-Type header to include the multipart boundary.

		case "application/xml":
			payloadMap, ok := options.payload.(map[string]interface{})
			if !ok {
				return nil, ErrInvalidPayloadTypeXML // Payload must be map[string]interface{} for XML.
			}
			xmlBytes, err := handleXMLData(payloadMap)
			if err != nil {
				return nil, fmt.Errorf("failed to handle XML data: %w", err)
			}
			requestBodyReader = bytes.NewReader(xmlBytes)

		default: // Default to application/json or handle user-defined Content-Type.
			// If Content-Type is something else (e.g., text/plain), payload should be []byte or string.
			// For this default case, we assume JSON if not specified, or try to marshal whatever payload is given.
			jsonBytes, err := json.Marshal(options.payload)
			if err != nil {
				return nil, fmt.Errorf("%w: marshalling payload to JSON (default type): %v", ErrMarshalPayload, err)
			}
			requestBodyReader = bytes.NewReader(jsonBytes)
			// If options.headers["Content-Type"] was empty, it will be set to "application/json" later.
		}
	}

	// Create the base *http.Request object.
	req, err := http.NewRequest(method, endpointURL, requestBodyReader)
	if err != nil {
		return nil, fmt.Errorf("%w: creating new HTTP request: %v", ErrHttpRequestCreate, err)
	}

	// Add all configured headers from options to the request.
	for k, v := range options.headers {
		req.Header.Add(k, v)
	}

	// Set default "Content-Type" to "application/json" if it's not already set by options
	// AND if there's a request body. This is a common default for POST/PUT/PATCH.
	if _, contentTypeHeaderExists := req.Header["Content-Type"]; !contentTypeHeaderExists {
		if requestBodyReader != nil { // Only add default Content-Type if there's actually a body.
			req.Header.Add("Content-Type", "application/json")
		}
	}

	// Add query parameters to the request URL.
	urlQueryValues := req.URL.Query()
	for k, v := range options.queries {
		urlQueryValues.Add(k, v)
	}
	req.URL.RawQuery = urlQueryValues.Encode()

	// If cache options were provided for this specific request (via WithCache),
	// update the client's cacheObj for the duration of this request execution.
	// This allows per-request cache configurations to override client-level defaults.
	if options.cacheObj != nil && options.cacheObj.fncs != nil {
		h.cacheObj = options.cacheObj
	}

	return req, nil
}

// executeRequest performs the HTTP request after it has been prepared by prepareRequest.
// This internal method is responsible for:
// - Logging the request execution time (profiling) using `defer h.profile()`.
// - Cache Read Logic:
//   - If caching is enabled for this request (h.cacheObj is set):
//     - A cache key is generated.
//     - It attempts to retrieve the response from the cache using `h.cacheObj.fncs.Get()`.
//     - If successful and the cached item is valid:
//       - The item is converted from its cached format (e.g., map) to an `*HttpResponse` using `toStruct`.
//       - If conversion is successful, the cached `*HttpResponse` is returned with `FromCache = true`.
//       - If conversion fails, an error is logged, and the flow proceeds as a cache miss (fetches from origin).
//     - If `h.cacheObj.fncs.Get()` itself fails (e.g., cache server error), an error is logged, and it proceeds as a cache miss.
// - Origin Fetch: If no valid cached response is found or caching is disabled, the request is made to the origin server using `h.client.Do()`.
// - Response Handling: The response body is read.
// - Cache Write Logic:
//   - If caching is enabled and the response from the origin is cacheable (e.g., HTTP 200 OK, 201 Created):
//     - The `*HttpResponse` is stored in the cache using `h.cacheObj.fncs.Set()`.
//     - Errors during cache set are logged but do not fail the overall request (the response from origin is still returned).
// Returns the `*HttpResponse` (either from cache or origin) and an error if the request to origin failed or critical unmarshalling errors occurred.
func (h *easyRequest) executeRequest(req *http.Request) (*HttpResponse, error) {
	defer h.profile(req.URL.Path, req.Method)() // Setup profiling for the request duration.

	// --- Cache Read Attempt ---
	if h.cacheObj != nil && h.cacheObj.fncs != nil {
		// Generate a unique cache key for this request using method, user-provided idempotency key, path, and query parameters.
		cacheKey := fmt.Sprintf("%s_%s_%s", req.Method, h.cacheObj.idempotency, fmt.Sprintf("%s?%s", req.URL.Path, req.URL.RawQuery))
		cachedItem, cacheGetErr := h.cacheObj.fncs.Get(cacheKey)

		if cacheGetErr == nil && cachedItem != nil { // Cache hit: item retrieved successfully.
			// Attempt to convert the cached item (which could be map[string]interface{} or similar)
			// back into a structured *HttpResponse.
			responseFromCache, conversionErr := toStruct[any, *HttpResponse](cachedItem)
			if conversionErr == nil { // Conversion successful.
				responseFromCache.cacheKey = cacheKey
				responseFromCache.FromCache = true
				return responseFromCache, nil // Return the valid response from cache.
			}
			// Conversion failed: Log the error. Treat this scenario as a cache miss and proceed to fetch from origin.
			switch logger := h.logger.(type) {
			case retryablehttp.LeveledLogger:
				logger.Error("Failed to convert cached data, fetching from origin", "key", cacheKey, "error", conversionErr)
			case retryablehttp.Logger:
				logger.Printf("Failed to convert cached data for key %s: %v. Fetching from origin.", cacheKey, conversionErr)
			}
		} else if cacheGetErr != nil { // Cache Get operation itself failed (e.g., connection error to cache server).
			// Log the cache get error. Proceed to fetch from origin.
			switch logger := h.logger.(type) {
			case retryablehttp.LeveledLogger:
				logger.Warn("Failed to get from cache, fetching from origin", "key", cacheKey, "error", cacheGetErr)
			case retryablehttp.Logger:
				logger.Printf("Cache get for key %s failed: %v. Fetching from origin.", cacheKey, cacheGetErr)
			}
		}
		// If cacheGetErr == nil and cachedItem == nil, it's a clean cache miss; flow proceeds to fetch from origin.
	}

	// --- Origin Fetch ---
	// Perform the HTTP request using the underlying (and potentially retryable) http.Client.
	originResp, err := h.client.Do(req)
	if err != nil {
		// Wrap the error from client.Do with more context.
		return nil, fmt.Errorf("%w: making request to origin server: %v", ErrHttpRequestDo, err)
	}
	defer originResp.Body.Close() // Ensure the response body is closed after the function returns.

	// Read the entire response body from the origin server.
	responseBodyBytes, err := io.ReadAll(originResp.Body)
	if err != nil {
		// If reading the body fails, still return an HttpResponse with status code and the error.
		return &HttpResponse{method: req.Method, StatusCode: originResp.StatusCode}, fmt.Errorf("%w: reading origin response body: %v", ErrReadResponseBody, err)
	}

	// Construct the final HttpResponse object from the origin's response.
	finalResponse := &HttpResponse{
		method:     req.Method,                 // HTTP method used.
		StatusCode: originResp.StatusCode,      // Status code from the server.
		Body:       responseBodyBytes,          // Raw response body.
		FromCache:  false,                      // Mark as not from cache, since it's fetched from origin.
	}

	// --- Cache Write Attempt ---
	// If caching is configured for this request (h.cacheObj) and the response is cacheable
	// (typically HTTP 200 OK or 201 Created statuses).
	if h.cacheObj != nil && h.cacheObj.fncs != nil &&
		(originResp.StatusCode == http.StatusOK || originResp.StatusCode == http.StatusCreated) {
		
		// Regenerate cache key for storing. It's crucial this key is identical to the one used for cache Get.
		cacheKey := fmt.Sprintf("%s_%s_%s", req.Method, h.cacheObj.idempotency, fmt.Sprintf("%s?%s", req.URL.Path, req.URL.RawQuery))
		finalResponse.cacheKey = cacheKey // Store the cache key in the response object.
		
		// Attempt to store the newly fetched response in the cache.
		if _, setErr := h.cacheObj.fncs.Set(cacheKey, finalResponse, h.cacheObj.expiry); setErr != nil {
			// Log any error during cache set. This failure does not fail the overall HTTP request,
			// as the primary goal (getting response from origin) was successful.
			wrappedSetErr := fmt.Errorf("%w: storing response for key %s in cache: %v", ErrCacheSet, cacheKey, setErr)
			switch logger := h.logger.(type) {
			case retryablehttp.LeveledLogger:
				logger.Error("Failed to cache response", "key", cacheKey, "error", wrappedSetErr)
			case retryablehttp.Logger:
				logger.Printf("Failed to cache response for key %s: %v", cacheKey, wrappedSetErr)
			}
		}
	}

	return finalResponse, nil
}

// Get performs an HTTP GET request to the client's base endpoint (concatenated with any path in opts).
// It accepts a variadic list of TReqOption functions to customize the request,
// such as adding headers (WithHeaders), query parameters (WithQueries), or enabling caching (WithCache).
// It returns an *HttpResponse containing the server's response (status, body, etc.)
// or an error if the request preparation or execution fails.
func (h *easyRequest) Get(opts ...TReqOption) (*HttpResponse, error) {
	// Prepare the HTTP GET request with the provided options.
	req, err := h.prepareRequest(http.MethodGet, h.endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("preparing GET request: %w", err)
	}
	// Execute the prepared request.
	return h.executeRequest(req)
}

// Post performs an HTTP POST request to the client's base endpoint.
// It accepts TReqOption functions for customization, most notably WithPayload to set the request body,
// WithHeaders to set "Content-Type" and other headers, and WithCache for caching.
// Returns an *HttpResponse or an error if request preparation or execution fails.
func (h *easyRequest) Post(opts ...TReqOption) (*HttpResponse, error) {
	// Prepare the HTTP POST request.
	req, err := h.prepareRequest(http.MethodPost, h.endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("preparing POST request: %w", err)
	}
	// Execute the prepared request.
	return h.executeRequest(req)
}

// Custom performs an HTTP request using a user-specified HTTP method (e.g., "PUT", "DELETE", "PATCH").
// It sends the request to the client's base endpoint.
// Accepts the HTTP method string and TReqOption functions for request customization.
// Returns an *HttpResponse or an error.
func (h *easyRequest) Custom(method string, opts ...TReqOption) (*HttpResponse, error) {
	// Prepare the custom HTTP request.
	req, err := h.prepareRequest(method, h.endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("preparing custom request (method %s): %w", method, err)
	}
	// Execute the prepared request.
	return h.executeRequest(req)
}

// Method returns the HTTP method (e.g., "GET", "POST") of the request
// that resulted in this HttpResponse.
func (h *HttpResponse) Method() string {
	return h.method
}

// CacheKey returns the key under which this response was (or would have been) stored in the cache.
// This key is generated based on request method, idempotency key, URL path, and query parameters.
// Returns an empty string if caching was not applicable or configured for the request.
func (h *HttpResponse) CacheKey() string {
	return h.cacheKey
}
