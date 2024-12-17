package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type TestData struct {
	Name  string `json:"name"    xml:"name"`
	Age   int    `json:"age"     xml:"age"`
	Email string `json:"email"   xml:"email"`
}

// Helper function to validate headers
func validateHeader(w http.ResponseWriter, r *http.Request, expectedHeader string) bool {
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, expectedHeader) {
		http.Error(w, fmt.Sprintf("Invalid Content-Type. Expected %s, got %s", expectedHeader, contentType), http.StatusBadRequest)
		return false
	}
	return true
}

// Unified handler for JSON
func jsonHandler(w http.ResponseWriter, r *http.Request) {
	if !validateHeader(w, r, "application/json") {
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		response := map[string]string{"status": "GET request received for /json"}
		json.NewEncoder(w).Encode(response)
	case http.MethodPost:
		var data TestData
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		response := map[string]string{
			"status": "success",
			"data":   fmt.Sprintf("Received JSON: %+v", data),
		}
		json.NewEncoder(w).Encode(response)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Unified handler for XML
func xmlHandler(w http.ResponseWriter, r *http.Request) {
	if !validateHeader(w, r, "application/xml") {
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/xml")
		response := "<response><status>GET request received for /xml</status></response>"
		w.Write([]byte(response))
	case http.MethodPost:
		var data TestData
		if err := xml.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, "Invalid XML", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		response := fmt.Sprintf("<response><status>success</status><data>Received XML: %+v</data></response>", data)
		w.Write([]byte(response))
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Unified handler for form-urlencoded
func formHandler(w http.ResponseWriter, r *http.Request) {
	if !validateHeader(w, r, "application/x-www-form-urlencoded") {
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("GET request received for /form"))
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}
		name := r.FormValue("name")
		age := r.FormValue("age")
		email := r.FormValue("email")
		response := fmt.Sprintf("Received Form Data: name=%s, age=%s, email=%s", name, age, email)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(response))
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Unified handler for multipart/form-data
func multipartHandler(w http.ResponseWriter, r *http.Request) {
	if !validateHeader(w, r, "multipart/form-data") {
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("GET request received for /multipart"))
	case http.MethodPost:
		if err := r.ParseMultipartForm(10 << 20); err != nil { // Limit max upload size to 10MB
			http.Error(w, "Invalid multipart form data", http.StatusBadRequest)
			return
		}
		name := r.FormValue("name")
		age := r.FormValue("age")
		email := r.FormValue("email")

		// Handling a file upload, if provided
		file, handler, err := r.FormFile("file")
		if err == nil {
			defer file.Close()
			dst, err := os.Create(handler.Filename)
			if err != nil {
				http.Error(w, "Failed to save file", http.StatusInternalServerError)
				return
			}
			defer dst.Close()
			io.Copy(dst, file)
		}

		response := fmt.Sprintf("Received Multipart Data: name=%s, age=%s, email=%s", name, age, email)
		if err == nil {
			response += fmt.Sprintf(", file=%s", handler.Filename)
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(response))
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func main() {
	port := 9000

	http.HandleFunc("/json", jsonHandler)           // Endpoint for application/json
	http.HandleFunc("/xml", xmlHandler)             // Endpoint for application/xml
	http.HandleFunc("/form", formHandler)           // Endpoint for application/x-www-form-urlencoded
	http.HandleFunc("/multipart", multipartHandler) // Endpoint for multipart/form-data

	fmt.Printf("Starting server on :%v...", port)
	err := http.ListenAndServe(fmt.Sprintf(":%v", port), nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
