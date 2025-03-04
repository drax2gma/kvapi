package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Version information - these values are injected during build
var (
	Version   = "development"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

const (
	MaxKeySize   = 255     // Maximum key size in bytes
	MaxValueSize = 1048576 // Maximum value size in bytes (1MB)
	MaxKeyCount  = 100     // Maximum number of keys allowed

	// ANSI color codes
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorYellow = "\033[33m"
	ColorGreen  = "\033[32m"
)

// KeyValueStore is a simple in-memory key-value store with mutex for concurrent access
type KeyValueStore struct {
	store map[string]string
	mu    sync.RWMutex
}

// StatusInfo represents the information returned by the status endpoint
type StatusInfo struct {
	KeyCount    int   `json:"key_count"`
	MemoryUsage int64 `json:"memory_usage_bytes"`
}

// AccessControl represents settings for controlling access to the API
type AccessControl struct {
	AllowedCIDR  *net.IPNet
	FirewallMode string // Can be "ACCEPT", "REJECT", or "DROP"
}

// APIResponse represents the standardized JSON response format
type APIResponse struct {
	Status    int         `json:"status"`
	Message   string      `json:"message"`
	Key       string      `json:"key,omitempty"`
	Value     string      `json:"value,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	TimeStamp string      `json:"timestamp"`
}

// NewKeyValueStore creates a new key-value store
func NewKeyValueStore() *KeyValueStore {
	return &KeyValueStore{
		store: make(map[string]string),
	}
}

// Get retrieves a value by key
func (kvs *KeyValueStore) Get(key string) (string, bool) {
	kvs.mu.RLock()
	defer kvs.mu.RUnlock()
	value, exists := kvs.store[key]
	return value, exists
}

// Set stores a key-value pair
// Returns error if the operation fails due to size or count constraints
func (kvs *KeyValueStore) Set(key, value string) error {
	// Check key size
	if len([]byte(key)) > MaxKeySize {
		return fmt.Errorf("key exceeds maximum size of %d bytes", MaxKeySize)
	}

	// Check value size
	if len([]byte(value)) > MaxValueSize {
		return fmt.Errorf("value exceeds maximum size of %d bytes", MaxValueSize)
	}

	kvs.mu.Lock()
	defer kvs.mu.Unlock()

	// Check if we're adding a new key and if we've reached the limit
	_, exists := kvs.store[key]
	if !exists && len(kvs.store) >= MaxKeyCount {
		return fmt.Errorf("maximum number of keys (%d) reached", MaxKeyCount)
	}

	kvs.store[key] = value
	return nil
}

// GetStatus returns information about the current state of the store
func (kvs *KeyValueStore) GetStatus() StatusInfo {
	kvs.mu.RLock()
	defer kvs.mu.RUnlock()

	var totalSize int64
	for k, v := range kvs.store {
		totalSize += int64(len([]byte(k)) + len([]byte(v)))
	}

	return StatusInfo{
		KeyCount:    len(kvs.store),
		MemoryUsage: totalSize,
	}
}

// getIPFromRequest extracts the client IP address from a request
func getIPFromRequest(r *http.Request) (net.IP, error) {
	// Get IP from RemoteAddr
	ipStr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return nil, err
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ipStr)
	}

	return ip, nil
}

// logMessage formats and prints a log message with timestamp and source IP
func logMessage(method, path, ip, msg string, rejected bool, statusCode ...int) {
	timestamp := time.Now().Format("2006-01-02T15:04:05.000-07:00")

	// Default colorization based on rejection status
	color := ColorReset
	if rejected {
		color = ColorYellow
	} else if len(statusCode) > 0 {
		// Apply colorization based on HTTP status code if provided
		status := statusCode[0]
		if status >= 200 && status < 300 {
			color = ColorGreen
		} else if status >= 300 && status < 500 {
			color = ColorRed
		} else if status >= 500 {
			color = ColorYellow
		}
	}

	if rejected {
		fmt.Printf("%s[%s] [REJECTED] %s %s from [%s] - %s%s\n",
			color, timestamp, method, path, ip, msg, ColorReset)
	} else {
		fmt.Printf("%s[%s] [%s] %s from [%s] - %s%s\n",
			color, timestamp, method, path, ip, msg, ColorReset)
	}
}

// accessMiddleware checks if the request IP is allowed based on CIDR restrictions
func accessMiddleware(ac *AccessControl, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// If no CIDR restrictions, allow all
		if ac.AllowedCIDR == nil {
			next(w, r)
			return
		}

		// Get client IP
		ip, err := getIPFromRequest(r)
		if err != nil {
			sendJSONResponse(w, http.StatusInternalServerError, "Failed to parse client IP", "", "", nil)
			timestamp := time.Now().Format("2006-01-02T15:04:05.000-07:00")
			fmt.Printf("[%s] Error parsing IP: %v\n", timestamp, err)
			return
		}

		// Check if IP is allowed
		if !ac.AllowedCIDR.Contains(ip) {
			// IP is not in allowed CIDR range - handle according to firewall mode
			switch ac.FirewallMode {
			case "DROP":
				// Simulate firewall DROP behavior but still log the attempt
				logMessage(r.Method, r.URL.Path, ip.String(), "DROPPED (fw-drop mode) - IP not in allowed CIDR", true, 0)
				// Don't respond to the client - terminate the connection silently
				// Using hijack to close the connection without sending a response
				hj, ok := w.(http.Hijacker)
				if ok {
					conn, _, _ := hj.Hijack()
					if conn != nil {
						conn.Close()
					}
				}
				return
			case "REJECT":
				// Simulate firewall REJECT behavior - actively refuse the connection
				logMessage(r.Method, r.URL.Path, ip.String(), "REJECTED (fw-reject mode) - IP not in allowed CIDR", true, http.StatusForbidden)
				// Send a "Connection Refused" type response
				sendJSONResponse(w, http.StatusForbidden, "Connection rejected by firewall: Your IP is not in the allowed range", "", "", nil)
				return
			default: // "ACCEPT" or any other value - standard 403 response
				// IP is not in allowed CIDR range - explicit reject with 403 Forbidden
				logMessage(r.Method, r.URL.Path, ip.String(), "Access denied (IP not in allowed CIDR)", true, http.StatusForbidden)
				sendJSONResponse(w, http.StatusForbidden, "Access denied: Your IP is not in the allowed range", "", "", nil)
				return
			}
		}

		// IP is allowed, proceed to next handler
		next(w, r)
	}
}

// sendJSONResponse sends a standardized JSON response
func sendJSONResponse(w http.ResponseWriter, status int, message string, key, value string, data interface{}) {
	response := APIResponse{
		Status:    status,
		Message:   message,
		TimeStamp: time.Now().Format(time.RFC3339),
	}

	// Add key and value for 200 responses
	if status == http.StatusOK && key != "" {
		response.Key = key
		response.Value = value
	}

	// Add additional data if provided
	if data != nil {
		response.Data = data
	}

	// Set content type and status code
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// Encode and send the response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// If JSON encoding fails, fall back to plain text
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error encoding JSON response"))
		fmt.Printf("Error encoding JSON response: %v\n", err)
	}
}

func main() {
	// Parse command line arguments
	listenAddr := flag.String("listen", ":8080", "Address and port to listen on (format: addr:port)")
	allowedCIDR := flag.String("allowed-cidr", "", "CIDR range for allowed IPs (e.g., 192.168.1.0/24). If not set, all IPs are allowed")
	fwDrop := flag.Bool("fw-drop", false, "If set, silently drops requests from non-allowed IPs (like a firewall DROP policy, with timeout)")
	fwReject := flag.Bool("fw-reject", false, "If set, actively rejects connections from non-allowed IPs (like a firewall REJECT policy)")
	udpMode := flag.Bool("udp", false, "Enable UDP mode instead of HTTP mode")
	showVersion := flag.Bool("version", false, "Show version information and exit")

	// For backward compatibility - to be deprecated
	simulateFirewall := flag.Bool("simulate-firewall", false, "Deprecated: Please use --fw-drop instead")

	// Override the default usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Key-Value API Server version %s (%s)\n\n", Version, GitCommit)
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nBuild time: %s\n", BuildTime)
	}

	flag.Parse()

	// Show version and exit if requested
	if *showVersion {
		fmt.Printf("Key-Value API Server version %s\n", Version)
		fmt.Printf("Git commit: %s\n", GitCommit)
		fmt.Printf("Build time: %s\n", BuildTime)
		os.Exit(0)
	}

	// Initialize access control
	var ac AccessControl

	// Prepare protocol type for display
	protocolType := "HTTP/TCP"
	if *udpMode {
		protocolType = "UDP"
	}

	// Display startup information with emojis
	fmt.Printf("\n🚀 Starting key-value API server v%s (%s) listening on %s\n", Version, GitCommit, *listenAddr)

	// Display applied rules based on command line switches
	fmt.Println("\n✨ === APPLIED RULES === ✨")

	// Network rules
	fmt.Println("🌐 Network rules:")
	fmt.Printf("  - Listen address: %s\n", *listenAddr)
	fmt.Printf("  - Protocol: %s\n", protocolType)

	// IP access rules
	fmt.Println("🔒 IP access rules:")
	if *allowedCIDR != "" {
		_, ipNet, err := net.ParseCIDR(*allowedCIDR)
		if err != nil {
			fmt.Printf("❌ Error parsing CIDR: %v\n", err)
			os.Exit(1)
		}
		ac.AllowedCIDR = ipNet
		fmt.Printf("  - Restricted to CIDR: %s\n", *allowedCIDR)

		// Handle firewall flags (set the FirewallMode to the appropriate value)
		// Support backward compatibility with --simulate-firewall as well
		if *fwDrop || *simulateFirewall {
			fmt.Printf("  - Firewall behavior: SILENTLY DROP non-matching IPs ⚠️\n")
			ac.FirewallMode = "DROP"
		} else if *fwReject {
			fmt.Printf("  - Firewall behavior: ACTIVELY REJECT non-matching IPs ⚠️\n")
			ac.FirewallMode = "REJECT"
		} else {
			fmt.Printf("  - Firewall behavior: 403 Forbidden response\n")
			ac.FirewallMode = "ACCEPT"
		}
	} else {
		fmt.Printf("  - All IP addresses allowed (no restrictions) ⚠️\n")
		fmt.Printf("  - Firewall behavior: ACCEPT ALL\n")
		ac.FirewallMode = "ACCEPT"
	}

	// Resource limits
	fmt.Println("📊 Resource limits:")
	fmt.Printf("  - Maximum keys: %d\n", MaxKeyCount)
	fmt.Printf("  - Maximum key size: %d bytes\n", MaxKeySize)
	fmt.Printf("  - Maximum value size: %d bytes (%d MB)\n", MaxValueSize, MaxValueSize/1024/1024)
	fmt.Printf("✨============================✨\n\n")

	// Create KeyValueStore
	kvs := NewKeyValueStore()

	// Start server based on mode
	if *udpMode {
		// Start UDP server
		fmt.Println("📡 UDP server is ready to accept connections! Press Ctrl+C to stop.")
		startUDPServer(*listenAddr, kvs, &ac)
	} else {
		// Continue with HTTP server setup
		mux := http.NewServeMux()

		// Ping endpoint
		mux.HandleFunc("/api/ping", accessMiddleware(&ac, func(w http.ResponseWriter, r *http.Request) {
			ip, _ := getIPFromRequest(r)
			ipStr := ip.String()

			if r.Method != http.MethodGet {
				logMessage(r.Method, r.URL.Path, ipStr, "Method not allowed", false, http.StatusMethodNotAllowed)
				sendJSONResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "", "", nil)
				return
			}

			logMessage(r.Method, r.URL.Path, ipStr, "PONG", false, http.StatusOK)
			sendJSONResponse(w, http.StatusOK, "PONG", "ping", "PONG", nil)
		}))

		// Status endpoint
		mux.HandleFunc("/api/status", accessMiddleware(&ac, func(w http.ResponseWriter, r *http.Request) {
			ip, _ := getIPFromRequest(r)
			ipStr := ip.String()

			if r.Method != http.MethodGet {
				logMessage(r.Method, r.URL.Path, ipStr, "Method not allowed", false, http.StatusMethodNotAllowed)
				sendJSONResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "", "", nil)
				return
			}

			status := kvs.GetStatus()
			logMessage(r.Method, r.URL.Path, ipStr, fmt.Sprintf("Status: %d keys, %d bytes", status.KeyCount, status.MemoryUsage), false, http.StatusOK)
			sendJSONResponse(w, http.StatusOK, "Status retrieved successfully", "status", "", status)
		}))

		// Get value endpoint
		mux.HandleFunc("/api/get", accessMiddleware(&ac, func(w http.ResponseWriter, r *http.Request) {
			ip, _ := getIPFromRequest(r)
			ipStr := ip.String()

			if r.Method != http.MethodGet {
				logMessage(r.Method, r.URL.Path, ipStr, "Method not allowed", false, http.StatusMethodNotAllowed)
				sendJSONResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "", "", nil)
				return
			}

			key := r.URL.Query().Get("k")
			if key == "" {
				logMessage(r.Method, r.URL.Path, ipStr, "Missing key parameter", false, http.StatusBadRequest)
				sendJSONResponse(w, http.StatusBadRequest, "Missing key parameter", "", "", nil)
				return
			}

			value, exists := kvs.Get(key)
			if !exists {
				logMessage(r.Method, r.URL.Path, ipStr, fmt.Sprintf("Key '%s' not found", key), false, http.StatusNotFound)
				sendJSONResponse(w, http.StatusNotFound, fmt.Sprintf("Key '%s' not found", key), key, "", nil)
				return
			}

			logMessage(r.Method, r.URL.Path, ipStr, fmt.Sprintf("Retrieved key '%s' with value '%s'", key, value), false, http.StatusOK)
			sendJSONResponse(w, http.StatusOK, "Key retrieved successfully", key, value, nil)
		}))

		// Set value endpoint
		mux.HandleFunc("/api/set", accessMiddleware(&ac, func(w http.ResponseWriter, r *http.Request) {
			ip, _ := getIPFromRequest(r)
			ipStr := ip.String()

			if r.Method != http.MethodPost && r.Method != http.MethodPut {
				logMessage(r.Method, r.URL.Path, ipStr, "Method not allowed", false, http.StatusMethodNotAllowed)
				sendJSONResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "", "", nil)
				return
			}

			key := r.URL.Query().Get("k")
			value := r.URL.Query().Get("v")

			if key == "" {
				logMessage(r.Method, r.URL.Path, ipStr, "Missing key parameter", false, http.StatusBadRequest)
				sendJSONResponse(w, http.StatusBadRequest, "Missing key parameter", "", "", nil)
				return
			}

			if value == "" {
				logMessage(r.Method, r.URL.Path, ipStr, "Missing value parameter", false, http.StatusBadRequest)
				sendJSONResponse(w, http.StatusBadRequest, "Missing value parameter", "", "", nil)
				return
			}

			err := kvs.Set(key, value)
			if err != nil {
				logMessage(r.Method, r.URL.Path, ipStr, fmt.Sprintf("Error setting key '%s': %v", key, err), false, http.StatusBadRequest)
				sendJSONResponse(w, http.StatusBadRequest, err.Error(), key, "", nil)
				return
			}

			logMessage(r.Method, r.URL.Path, ipStr, fmt.Sprintf("Set key '%s' to value '%s'", key, value), false, http.StatusOK)
			sendJSONResponse(w, http.StatusOK, "Key set successfully", key, value, nil)
		}))

		// NotFound handler for logging 404 requests
		notFoundHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, err := getIPFromRequest(r)
			ipStr := "unknown"
			if err == nil {
				ipStr = ip.String()
			}
			logMessage(r.Method, r.URL.Path, ipStr, "Route not found", false, http.StatusNotFound)

			// Return JSON response for 404 to maintain consistent API response format
			sendJSONResponse(w, http.StatusNotFound, fmt.Sprintf("Route '%s' not found", r.URL.Path), "", "", nil)
		})

		// Create a middleware to catch all requests
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Use the mux to find a handler, or use notFoundHandler if none exists
			h, pattern := mux.Handler(r)
			if pattern == "" {
				// No handler found, use our custom 404 handler
				notFoundHandler.ServeHTTP(w, r)
				return
			}
			// Handler found, use it
			h.ServeHTTP(w, r)
		})

		// Start server with our custom handler
		fmt.Println("📡 HTTP server is ready to accept connections! Press Ctrl+C to stop.")
		log.Fatal(http.ListenAndServe(*listenAddr, handler))
	}
}

// handleUDPCommand processes a UDP command and returns a response
func handleUDPCommand(command string, addr net.Addr, kvs *KeyValueStore, ac *AccessControl) []byte {
	// Extract client IP for access control and logging
	ipStr := strings.Split(addr.String(), ":")[0]
	ip := net.ParseIP(ipStr)

	// Check IP restrictions if CIDR is set
	if ac.AllowedCIDR != nil && !ac.AllowedCIDR.Contains(ip) {
		// Handle based on firewall mode
		switch ac.FirewallMode {
		case "DROP":
			// Log the dropped packet but return nil (no response)
			logMessage("UDP", "command", ipStr, "DROPPED (fw-drop mode) - IP not in allowed CIDR", true, 0)
			return nil
		case "REJECT":
			// Log the rejected packet and send a rejection response
			logMessage("UDP", "command", ipStr, "REJECTED (fw-reject mode) - IP not in allowed CIDR", true, http.StatusForbidden)
			response := APIResponse{
				Status:    http.StatusForbidden,
				Message:   "Connection rejected by firewall: Your IP is not in the allowed range",
				TimeStamp: time.Now().Format(time.RFC3339),
			}
			jsonResponse, _ := json.Marshal(response)
			return jsonResponse
		default: // "ACCEPT" or any other value
			logMessage("UDP", "command", ipStr, "Access denied (IP not in allowed CIDR)", true, http.StatusForbidden)
			response := APIResponse{
				Status:    http.StatusForbidden,
				Message:   "Access denied: Your IP is not in the allowed range",
				TimeStamp: time.Now().Format(time.RFC3339),
			}
			jsonResponse, _ := json.Marshal(response)
			return jsonResponse
		}
	}

	// Split the command into parts
	parts := strings.Fields(command)
	if len(parts) == 0 {
		logMessage("UDP", "command", ipStr, "Empty command", false, http.StatusBadRequest)
		response := APIResponse{
			Status:    http.StatusBadRequest,
			Message:   "Empty command",
			TimeStamp: time.Now().Format(time.RFC3339),
		}
		jsonResponse, _ := json.Marshal(response)
		return jsonResponse
	}

	action := strings.ToUpper(parts[0])

	// Process command based on action
	switch action {
	case "PING":
		logMessage("UDP", "PING", ipStr, "PONG", false, http.StatusOK)
		response := APIResponse{
			Status:    http.StatusOK,
			Message:   "PONG",
			Key:       "ping",
			Value:     "PONG",
			TimeStamp: time.Now().Format(time.RFC3339),
		}
		jsonResponse, _ := json.Marshal(response)
		return jsonResponse

	case "STATUS":
		status := kvs.GetStatus()
		logMessage("UDP", "STATUS", ipStr, fmt.Sprintf("Status: %d keys, %d bytes", status.KeyCount, status.MemoryUsage), false, http.StatusOK)
		response := APIResponse{
			Status:    http.StatusOK,
			Message:   "Status retrieved successfully",
			Key:       "status",
			Data:      status,
			TimeStamp: time.Now().Format(time.RFC3339),
		}
		jsonResponse, _ := json.Marshal(response)
		return jsonResponse

	case "GET":
		if len(parts) < 2 {
			logMessage("UDP", "GET", ipStr, "Missing key parameter", false, http.StatusBadRequest)
			response := APIResponse{
				Status:    http.StatusBadRequest,
				Message:   "Missing key parameter",
				TimeStamp: time.Now().Format(time.RFC3339),
			}
			jsonResponse, _ := json.Marshal(response)
			return jsonResponse
		}

		key := parts[1]
		value, exists := kvs.Get(key)

		if !exists {
			logMessage("UDP", "GET", ipStr, fmt.Sprintf("Key '%s' not found", key), false, http.StatusNotFound)
			response := APIResponse{
				Status:    http.StatusNotFound,
				Message:   fmt.Sprintf("Key '%s' not found", key),
				Key:       key,
				TimeStamp: time.Now().Format(time.RFC3339),
			}
			jsonResponse, _ := json.Marshal(response)
			return jsonResponse
		}

		logMessage("UDP", "GET", ipStr, fmt.Sprintf("Retrieved key '%s' with value '%s'", key, value), false, http.StatusOK)
		response := APIResponse{
			Status:    http.StatusOK,
			Message:   "Key retrieved successfully",
			Key:       key,
			Value:     value,
			TimeStamp: time.Now().Format(time.RFC3339),
		}
		jsonResponse, _ := json.Marshal(response)
		return jsonResponse

	case "SET":
		if len(parts) < 2 {
			logMessage("UDP", "SET", ipStr, "Missing key parameter", false, http.StatusBadRequest)
			response := APIResponse{
				Status:    http.StatusBadRequest,
				Message:   "Missing key parameter",
				TimeStamp: time.Now().Format(time.RFC3339),
			}
			jsonResponse, _ := json.Marshal(response)
			return jsonResponse
		}

		if len(parts) < 3 {
			logMessage("UDP", "SET", ipStr, "Missing value parameter", false, http.StatusBadRequest)
			response := APIResponse{
				Status:    http.StatusBadRequest,
				Message:   "Missing value parameter",
				TimeStamp: time.Now().Format(time.RFC3339),
			}
			jsonResponse, _ := json.Marshal(response)
			return jsonResponse
		}

		key := parts[1]
		// Join the rest of the parts as the value (in case it contains spaces)
		value := strings.Join(parts[2:], " ")

		err := kvs.Set(key, value)
		if err != nil {
			logMessage("UDP", "SET", ipStr, fmt.Sprintf("Error setting key '%s': %v", key, err), false, http.StatusBadRequest)
			response := APIResponse{
				Status:    http.StatusBadRequest,
				Message:   err.Error(),
				Key:       key,
				TimeStamp: time.Now().Format(time.RFC3339),
			}
			jsonResponse, _ := json.Marshal(response)
			return jsonResponse
		}

		logMessage("UDP", "SET", ipStr, fmt.Sprintf("Set key '%s' to value '%s'", key, value), false, http.StatusOK)
		response := APIResponse{
			Status:    http.StatusOK,
			Message:   "Key set successfully",
			Key:       key,
			Value:     value,
			TimeStamp: time.Now().Format(time.RFC3339),
		}
		jsonResponse, _ := json.Marshal(response)
		return jsonResponse

	default:
		logMessage("UDP", action, ipStr, "Unknown command", false, http.StatusBadRequest)
		response := APIResponse{
			Status:    http.StatusBadRequest,
			Message:   fmt.Sprintf("Unknown command: %s", action),
			TimeStamp: time.Now().Format(time.RFC3339),
		}
		jsonResponse, _ := json.Marshal(response)
		return jsonResponse
	}
}

// startUDPServer starts a UDP server on the given address
func startUDPServer(listenAddr string, kvs *KeyValueStore, ac *AccessControl) {
	addr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		log.Fatalf("Failed to resolve UDP address: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("Failed to start UDP server: %v", err)
	}
	defer conn.Close()

	log.Printf("UDP server listening on %s", listenAddr)

	buffer := make([]byte, 8192) // 8KB buffer for UDP packets

	for {
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Error reading from UDP: %v", err)
			continue
		}

		command := string(buffer[:n])
		command = strings.TrimSpace(command)

		// Handle the command
		response := handleUDPCommand(command, clientAddr, kvs, ac)

		// Send the response back to the client
		_, err = conn.WriteToUDP(response, clientAddr)
		if err != nil {
			log.Printf("Error sending UDP response: %v", err)
		}
	}
}
