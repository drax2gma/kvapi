// Package main provides a command-line client for the key-value API server
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Version information - these values are injected during build
var (
	Version   = "development"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// Response represents the standard server response structure
type Response struct {
	Status    int                    `json:"status"`
	Message   string                 `json:"message"`
	Key       string                 `json:"key,omitempty"`
	Value     string                 `json:"value,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp string                 `json:"timestamp"`
}

// Options holds the client configuration
type Options struct {
	Host     string
	Port     int
	Protocol string
	Timeout  time.Duration
}

func main() {
	// Define command-line flags
	protocol := flag.String("protocol", "http", "Protocol to use (http or udp)")
	host := flag.String("host", "localhost", "Server hostname or IP address")
	port := flag.Int("port", 8080, "Server port")
	timeout := flag.Float64("timeout", 2.0, "Timeout in seconds")
	showVersion := flag.Bool("version", false, "Show version information and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Key-Value API Client v%s (%s)\n\n", Version, GitCommit)
		fmt.Fprintf(os.Stderr, "Usage: kvclient [options] <command> [args...]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nCommands:\n")
		fmt.Fprintf(os.Stderr, "  PING                        Check if the server is up\n")
		fmt.Fprintf(os.Stderr, "  STATUS                      Get server status information\n")
		fmt.Fprintf(os.Stderr, "  GET <key>                   Retrieve a value by key\n")
		fmt.Fprintf(os.Stderr, "  SET <key> <value>           Set a key-value pair\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  kvclient PING\n")
		fmt.Fprintf(os.Stderr, "  kvclient -protocol=udp -port=4000 STATUS\n")
		fmt.Fprintf(os.Stderr, "  kvclient GET mykey\n")
		fmt.Fprintf(os.Stderr, "  kvclient SET greeting \"Hello, World!\"\n")
		fmt.Fprintf(os.Stderr, "\nBuild time: %s\n", BuildTime)
	}
	flag.Parse()

	// Show version and exit if requested
	if *showVersion {
		fmt.Printf("Key-Value API Client v%s\n", Version)
		fmt.Printf("Git commit: %s\n", GitCommit)
		fmt.Printf("Build time: %s\n", BuildTime)
		os.Exit(0)
	}

	// Validate protocol
	if *protocol != "http" && *protocol != "udp" {
		fmt.Fprintf(os.Stderr, "Error: Protocol must be 'http' or 'udp'\n")
		flag.Usage()
		os.Exit(1)
	}

	// Validate port
	if *port < 1 || *port > 65535 {
		fmt.Fprintf(os.Stderr, "Error: Port must be between 1 and 65535\n")
		flag.Usage()
		os.Exit(1)
	}

	// Set up client options
	opts := Options{
		Host:     *host,
		Port:     *port,
		Protocol: *protocol,
		Timeout:  time.Duration(*timeout * float64(time.Second)),
	}

	// Parse command
	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: No command specified\n")
		flag.Usage()
		os.Exit(1)
	}

	command := strings.ToUpper(args[0])
	cmdArgs := args[1:]

	// Execute command
	var response *Response
	var err error

	fmt.Printf("🔌 Key-Value Client v%s connecting to %s server at %s:%d...\n",
		Version, strings.ToUpper(*protocol), *host, *port)

	switch command {
	case "PING":
		response, err = ping(opts)
	case "STATUS":
		response, err = status(opts)
	case "GET":
		if len(cmdArgs) < 1 {
			fmt.Fprintf(os.Stderr, "Error: GET command requires a key\n")
			flag.Usage()
			os.Exit(1)
		}
		response, err = get(opts, cmdArgs[0])
	case "SET":
		if len(cmdArgs) < 2 {
			fmt.Fprintf(os.Stderr, "Error: SET command requires a key and a value\n")
			flag.Usage()
			os.Exit(1)
		}
		value := strings.Join(cmdArgs[1:], " ")
		response, err = set(opts, cmdArgs[0], value)
	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown command: %s\n", command)
		flag.Usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: %s\n", err)
		os.Exit(1)
	}

	// Print response with proper formatting and colors
	printResponse(response)
}

// ping checks if the server is up
func ping(opts Options) (*Response, error) {
	if opts.Protocol == "udp" {
		return sendUDPCommand(opts, "PING")
	}
	return sendHTTPRequest(opts, "ping", "GET", nil)
}

// status gets server status information
func status(opts Options) (*Response, error) {
	if opts.Protocol == "udp" {
		return sendUDPCommand(opts, "STATUS")
	}
	return sendHTTPRequest(opts, "status", "GET", nil)
}

// get retrieves a value by key
func get(opts Options, key string) (*Response, error) {
	if opts.Protocol == "udp" {
		return sendUDPCommand(opts, fmt.Sprintf("GET %s", key))
	}

	params := url.Values{}
	params.Set("k", key)
	return sendHTTPRequest(opts, "get", "GET", params)
}

// set sets a key-value pair
func set(opts Options, key, value string) (*Response, error) {
	if opts.Protocol == "udp" {
		return sendUDPCommand(opts, fmt.Sprintf("SET %s %s", key, value))
	}

	params := url.Values{}
	params.Set("k", key)
	params.Set("v", value)
	return sendHTTPRequest(opts, "set", "POST", params)
}

// sendUDPCommand sends a command to the UDP server
func sendUDPCommand(opts Options, command string) (*Response, error) {
	fmt.Printf("📤 Sending UDP command: %s\n", command)

	// Create UDP address
	addr := fmt.Sprintf("%s:%d", opts.Host, opts.Port)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address: %w", err)
	}

	// Create UDP socket and set timeout
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	// Set timeout
	conn.SetDeadline(time.Now().Add(opts.Timeout))

	// Send command
	_, err = conn.Write([]byte(command))
	if err != nil {
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	// Receive response
	buffer := make([]byte, 8192)
	n, _, err := conn.ReadFromUDP(buffer)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil, fmt.Errorf("request timed out after %.1f seconds", opts.Timeout.Seconds())
		}
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}

	// Parse JSON response
	var response Response
	err = json.Unmarshal(buffer[:n], &response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// sendHTTPRequest sends a request to the HTTP server
func sendHTTPRequest(opts Options, endpoint string, method string, params url.Values) (*Response, error) {
	// Create base URL
	baseURL := fmt.Sprintf("http://%s:%d/api/%s", opts.Host, opts.Port, endpoint)

	// Create request
	var req *http.Request
	var err error

	if params != nil {
		if method == "GET" {
			// For GET, append parameters to URL
			reqURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
			fmt.Printf("📤 Sending HTTP %s request: %s\n", method, reqURL)
			req, err = http.NewRequest(method, reqURL, nil)
		} else {
			// For POST/PUT, add as form data
			fmt.Printf("📤 Sending HTTP %s request to %s with parameters: %s\n", method, baseURL, params.Encode())
			req, err = http.NewRequest(method, baseURL, bytes.NewBufferString(params.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	} else {
		fmt.Printf("📤 Sending HTTP %s request: %s\n", method, baseURL)
		req, err = http.NewRequest(method, baseURL, nil)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: opts.Timeout,
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, fmt.Errorf("request timed out after %.1f seconds", opts.Timeout.Seconds())
		}
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse JSON response
	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// printResponse prints the response in a nicely formatted way
func printResponse(resp *Response) {
	// Get status color
	var statusColor string
	var statusText string

	switch {
	case resp.Status >= 200 && resp.Status < 300:
		statusColor = "\033[32m" // Green
		statusText = "OK"
	case resp.Status >= 300 && resp.Status < 400:
		statusColor = "\033[33m" // Yellow
		statusText = "REDIRECT"
	case resp.Status >= 400 && resp.Status < 500:
		statusColor = "\033[31m" // Red
		statusText = "CLIENT ERROR"
	case resp.Status >= 500:
		statusColor = "\033[31m" // Red
		statusText = "SERVER ERROR"
	default:
		statusColor = "\033[0m" // Reset
		statusText = "UNKNOWN"
	}

	resetColor := "\033[0m"

	// Print status line
	fmt.Printf("\n📥 Response received:\n")
	fmt.Printf("Status: %s%d %s%s\n", statusColor, resp.Status, statusText, resetColor)
	fmt.Printf("Message: %s\n", resp.Message)

	// Print key and value if present
	if resp.Key != "" {
		fmt.Printf("Key: %s\n", resp.Key)
	}
	if resp.Value != "" {
		fmt.Printf("Value: %s\n", resp.Value)
	}

	// Print data if present
	if resp.Data != nil && len(resp.Data) > 0 {
		fmt.Printf("Data:\n")
		jsonData, err := json.MarshalIndent(resp.Data, "  ", "  ")
		if err != nil {
			fmt.Printf("  Error formatting data: %s\n", err)
		} else {
			fmt.Printf("  %s\n", jsonData)
		}
	}

	// Print timestamp
	fmt.Printf("Timestamp: %s\n", resp.Timestamp)
}
