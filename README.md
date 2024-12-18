# easyrqst

A simple and efficient HTTP request library for Go.

## Features

- Easy-to-use HTTP client wrapper
- Support for common HTTP methods (GET, POST, PUT, DELETE, etc.)
- Chainable request building
- Custom header support
- Request timeout configuration
- Error handling simplified

## Installation

```bash
go get github.com/captain-bugs/easyrqst
```

## Usage

### Example Usage

#### GET Request

```go
package main

import (
    "fmt"
    "log"
    "time"

    "github.com/captain-bugs/easyrqst"
)

func main() {
    const endpoint = "http://localhost:9000/json"
    call := easyrqst.NewHttpClient(endpoint, easyrqst.WithRetry(4), easyrqst.WithRetryWaitMax(time.Millisecond*100))
    headers := easyrqst.WithHeaders(map[string]string{"Content-Type": "application/json"})

    outcome, err := call.Get(headers)
    if err != nil {
        log.Fatalf("Error: %v", err)
    }
    if outcome.StatusCode != 200 {
        log.Fatalf("Expected status code 200, got %v", outcome.StatusCode)
    }
    fmt.Println(string(outcome.Body))
}
```

#### POST Request with JSON Payload

```go
package main

import (
    "fmt"
    "log"
    "time"

    "github.com/captain-bugs/easyrqst"
)

func main() {
    const endpoint = "http://localhost:9000/json"
    call := easyrqst.NewHttpClient(endpoint, easyrqst.WithRetry(4), easyrqst.WithRetryWaitMax(time.Millisecond*100))

    body := easyrqst.WithPayload(map[string]any{"name": "morpheus", "age": 30, "email": "example@example.com"})
    headers := easyrqst.WithHeaders(map[string]string{"Content-Type": "application/json"})

    outcome, err := call.Post(body, headers)
    if err != nil {
        log.Fatalf("Error: %v", err)
    }
    if outcome.StatusCode != 201 {
        log.Fatalf("Expected status code 201, got %v", outcome.StatusCode)
    }
    fmt.Println(string(outcome.Body))
}
```

#### POST Request with Multipart Form Data

```go
package main

import (
    "fmt"
    "log"
    "time"

    "github.com/captain-bugs/easyrqst"
)

func main() {
    const endpoint = "http://localhost:9000/multipart"
    call := easyrqst.NewHttpClient(endpoint, easyrqst.WithRetry(4), easyrqst.WithRetryWaitMax(time.Millisecond*100))

    body := easyrqst.WithPayload(map[string]string{"name": "morpheus", "age": "30", "email": "example@example.com"})
    headers := easyrqst.WithHeaders(map[string]string{"Content-Type": "multipart/form-data"})
    files := easyrqst.WithFiles(map[string]string{"files": "example/img.png"})

    outcome, err := call.Post(body, headers, files)
    if err != nil {
        log.Fatalf("Error: %v", err)
    }
    if outcome.StatusCode != 201 {
        log.Fatalf("Expected status code 201, got %v", outcome.StatusCode)
    }
    fmt.Println(string(outcome.Body))
}
```

#### GET Request with XML Response

```go
package main

import (
    "fmt"
    "log"
    "time"

    "github.com/captain-bugs/easyrqst"
)

func main() {
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
    fmt.Println(string(outcome.Body))
}
```

#### POST Request with XML Payload

```go
package main

import (
    "fmt"
    "log"
    "time"

    "github.com/captain-bugs/easyrqst"
)

func main() {
    const endpoint = "http://localhost:9000/xml"
    call := easyrqst.NewHttpClient(endpoint, easyrqst.WithRetry(4), easyrqst.WithRetryWaitMax(time.Millisecond*100))

    body := easyrqst.WithPayload(map[string]interface{}{"person": map[string]interface{}{"name": "John Doe", "age": "30", "address": map[string]interface{}{"city": "New York", "state": "NY"}}})
    headers := easyrqst.WithHeaders(map[string]string{"Content-Type": "application/xml"})

    outcome, err := call.Post(body, headers)
    if err != nil {
        log.Fatalf("Error: %v", err)
    }
    if outcome.StatusCode != 201 {
        log.Fatalf("Expected status code 201, got %v", outcome.StatusCode)
    }
    fmt.Println(string(outcome.Body))
}
```

#### GET Request with Form URL Encoded Response

```go
package main

import (
    "fmt"
    "log"
    "time"

    "github.com/captain-bugs/easyrqst"
)

func main() {
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
    fmt.Println(string(outcome.Body))
}
```

#### POST Request with Form URL Encoded Payload

```go
package main

import (
    "fmt"
    "log"
    "time"

    "github.com/captain-bugs/easyrqst"
)

func main() {
    const endpoint = "http://localhost:9000/form"
    call := easyrqst.NewHttpClient(endpoint, easyrqst.WithRetry(4), easyrqst.WithRetryWaitMax(time.Millisecond*100))

    body := easyrqst.WithPayload(map[string]string{"name": "morpheus", "age": "30", "email": "example@example.com"})
    headers := easyrqst.WithHeaders(map[string]string{"Content-Type": "application/x-www-form-urlencoded"})

    outcome, err := call.Post(body, headers)
    if err != nil {
        log.Fatalf("Error: %v", err)
    }
    if outcome.StatusCode != 201 {
        log.Fatalf("Expected status code 201, got %v", outcome.StatusCode)
    }
    fmt.Println(string(outcome.Body))
}
```


## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Author

Captain Bugs (@[captain-bugs](https://github.com/captain-bugs))
