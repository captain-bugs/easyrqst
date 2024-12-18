package easyrqst

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/hashicorp/go-retryablehttp"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type xmlElement struct {
	XMLName  xml.Name
	Attr     []xml.Attr
	Content  string       `xml:",chardata"`
	Children []xmlElement `xml:",any"`
}

type ICacheFn interface {
	Get(key string) (any, error)
	Set(key string, value any, expiry time.Duration) (any, error)
	Delete(key string) error
}

type IHttpClient interface {
	Get(opts ...TReqOption) (*HttpResponse, error)
	Post(opts ...TReqOption) (*HttpResponse, error)
	Custom(method string, opts ...TReqOption) (*HttpResponse, error)
}

type TReqOption func(*ReqOptions)
type THttpOption func(*easyRequest)

type cacheObj struct {
	fncs        ICacheFn
	expiry      time.Duration
	idempotency string
}

type ReqOptions struct {
	queries  map[string]string
	headers  map[string]string
	files    map[string]string
	cacheObj *cacheObj
	payload  any
}

type easyRequest struct {
	forceCache   bool
	cacheObj     *cacheObj
	endpoint     string
	client       *http.Client
	maxRetry     int
	retryWaitMax time.Duration
	logger       interface{}
}

type HttpResponse struct {
	method     string
	cacheKey   string
	FromCache  bool
	StatusCode int
	Body       []byte
}

func handleMultipartFormData(payload map[string]string, files map[string]string) (*bytes.Buffer, string, error) {
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)

	for key, val := range payload {
		err := writer.WriteField(key, val)
		if err != nil {
			return nil, "", err
		}
	}

	for filename, filePath := range files {
		file, err := os.Open(filePath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to open file %s: %v", filePath, err)
		}
		defer file.Close()

		part, err := writer.CreateFormFile(filename, filepath.Base(filePath))
		if err != nil {
			return nil, "", err
		}

		_, err = io.Copy(part, file)
		if err != nil {
			return nil, "", err
		}
	}

	err := writer.Close()
	if err != nil {
		return nil, "", err
	}

	return &b, writer.FormDataContentType(), nil
}

func convertToXMLElements(data map[string]interface{}) []xmlElement {
	var elements []xmlElement
	for key, value := range data {
		element := xmlElement{
			XMLName: xml.Name{Local: key},
		}
		switch v := value.(type) {
		case string:
			element.Content = v
		case map[string]interface{}:
			element.Children = convertToXMLElements(v)
		default:
			element.Content = fmt.Sprintf("%v", v)
		}
		elements = append(elements, element)
	}
	return elements
}

func handleXMLData(data map[string]interface{}) ([]byte, error) {
	return xml.MarshalIndent(convertToXMLElements(data), "", "  ")
}

func toStruct[M any, S any](m M) S {
	data, _ := json.Marshal(m)
	var result S
	_ = json.Unmarshal(data, &result)
	return result
}

func WithQueries(queries map[string]string) TReqOption {
	return func(o *ReqOptions) { o.queries = queries }
}

func WithHeaders(headers map[string]string) TReqOption {
	return func(o *ReqOptions) { o.headers = headers }
}

func WithPayload(payload any) TReqOption {
	return func(o *ReqOptions) { o.payload = payload }
}

func WithFiles(files map[string]string) TReqOption {
	return func(o *ReqOptions) { o.files = files }
}

func WithCache(cache ICacheFn, period time.Duration, idempotency string) TReqOption {
	return func(o *ReqOptions) {
		o.cacheObj = &cacheObj{fncs: cache, expiry: period, idempotency: idempotency}
	}
}

func WithRetry(max int) THttpOption {
	return func(o *easyRequest) { o.maxRetry = max }
}

func WithRetryWaitMax(wait time.Duration) THttpOption {
	return func(o *easyRequest) { o.retryWaitMax = wait }
}

func WithLogger(logger interface{}) THttpOption {
	return func(o *easyRequest) { o.logger = logger }
}

func NewHttpClient(endpoint string, opts ...THttpOption) IHttpClient {
	client := retryablehttp.NewClient()
	easyRqstClient := &easyRequest{
		endpoint:     endpoint,
		client:       client.StandardClient(),
		maxRetry:     3,
		retryWaitMax: 1 * time.Second,
		logger:       nil,
	}
	for _, opt := range opts {
		opt(easyRqstClient)
	}
	client.RetryMax = easyRqstClient.maxRetry
	client.RetryWaitMax = easyRqstClient.retryWaitMax
	client.Logger = easyRqstClient.logger

	return easyRqstClient
}

func (h *easyRequest) profile(url, method string) func() {
	start := time.Now()
	return func() {
		ms := time.Since(start).String()
		switch v := h.logger.(type) {
		case retryablehttp.LeveledLogger:
			v.Debug("REQUEST_TIME", "url", url, "method", method, "elapsed", ms)
		case retryablehttp.Logger:
			v.Printf("REQUEST_TIME url=%s method=%s elapsed=%v", url, method, ms)
		}
	}
}

func (h *easyRequest) prepareRequest(method, endpoint string, opts ...TReqOption) (*http.Request, error) {
	options := ReqOptions{
		queries: make(map[string]string),
		headers: make(map[string]string),
	}

	// Apply options
	for _, opt := range opts {
		opt(&options)
	}

	var body io.Reader
	// Handle payload based on content type
	if options.payload != nil || options.files != nil {
		switch options.headers["Content-Type"] {

		case "application/x-www-form-urlencoded":
			data := url.Values{}
			if formData, ok := options.payload.(map[string]string); ok {
				for k, v := range formData {
					data.Set(k, v)
				}
				body = bytes.NewReader([]byte(data.Encode()))
			} else {
				return nil, fmt.Errorf("payload should be a map[string]string for x-www-form-urlencoded")
			}

		case "multipart/form-data":
			if _, ok := options.payload.(map[string]string); !ok {
				return nil, fmt.Errorf("payload should be a map[string]string for multipart/form-data")
			}
			b, contentType, err := handleMultipartFormData(options.payload.(map[string]string), options.files)
			if err != nil {
				return nil, err
			}
			body = b
			options.headers["Content-Type"] = contentType

		case "application/xml":
			if _, ok := options.payload.(map[string]interface{}); !ok {
				return nil, fmt.Errorf("payload should be a map[string]interface{} for application/xml")
			}
			byts, err := handleXMLData(options.payload.(map[string]interface{}))
			if err != nil {
				return nil, err
			}
			body = bytes.NewReader(byts)

		default:
			byts, err := json.Marshal(options.payload)
			if err != nil {
				return nil, err
			}
			body = bytes.NewReader(byts)
		}
	}

	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, err
	}

	// Add headers
	for k, v := range options.headers {
		req.Header.Add(k, v)
	}

	// Default content type
	if _, exist := options.headers["Content-Type"]; !exist {
		req.Header.Add("Content-Type", "application/json")
	}

	// Add queries
	query := req.URL.Query()
	for k, v := range options.queries {
		query.Add(k, v)
	}
	req.URL.RawQuery = query.Encode()

	if options.cacheObj != nil && options.cacheObj.fncs != nil {
		h.cacheObj = options.cacheObj
	}

	return req, nil
}

func (h *easyRequest) executeRequest(req *http.Request) (*HttpResponse, error) {
	defer h.profile(req.URL.Path, req.Method)()

	if h.cacheObj != nil && h.cacheObj.fncs != nil {
		key := fmt.Sprintf("%s_%s_%s", req.Method, h.cacheObj.idempotency, fmt.Sprintf("%s?%s", req.URL.Path, req.URL.RawQuery))
		if cached, err := h.cacheObj.fncs.Get(key); err == nil {
			data := toStruct[any, *HttpResponse](cached)
			data.cacheKey = key
			data.FromCache = true
			return data, nil
		}
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &HttpResponse{method: req.Method, StatusCode: resp.StatusCode}, err
	}

	response := &HttpResponse{method: req.Method, StatusCode: resp.StatusCode, Body: body}

	if h.cacheObj != nil && h.cacheObj.fncs != nil && (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated) {
		response.FromCache = false
		response.cacheKey = fmt.Sprintf("%s_%s_%s", req.Method, h.cacheObj.idempotency, fmt.Sprintf("%s?%s", req.URL.Path, req.URL.RawQuery))
		_, err = h.cacheObj.fncs.Set(response.cacheKey, response, h.cacheObj.expiry)
	}

	return response, nil
}

func (h *easyRequest) Get(opts ...TReqOption) (*HttpResponse, error) {
	req, err := h.prepareRequest(http.MethodGet, h.endpoint, opts...)
	if err != nil {
		return nil, err
	}
	return h.executeRequest(req)
}

func (h *easyRequest) Post(opts ...TReqOption) (*HttpResponse, error) {
	req, err := h.prepareRequest(http.MethodPost, h.endpoint, opts...)
	if err != nil {
		return nil, err
	}
	return h.executeRequest(req)
}

func (h *easyRequest) Custom(method string, opts ...TReqOption) (*HttpResponse, error) {
	req, err := h.prepareRequest(method, h.endpoint, opts...)
	if err != nil {
		return nil, err
	}
	return h.executeRequest(req)
}

func (h *HttpResponse) Method() string {
	return h.method
}

func (h *HttpResponse) CacheKey() string {
	return h.cacheKey
}
