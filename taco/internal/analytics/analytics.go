package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/storage"
)

type Level uint8

const (
	Off Level = iota
	Essential
	Reduced
	Default
)

// Constants for configuration
const (
	// Analytics service endpoint
	DefaultAnalyticsEndpoint = "https://analytics-1035733479708.us-east1.run.app"
	
	// Timeouts
	DefaultRequestTimeout = 5 * time.Second
	AsyncRequestTimeout   = 10 * time.Second
	
	// Buffer sizes
	SystemIDBufferSize = 32  // 16 bytes = 32 hex chars
	EmailBufferSize    = 256 // Reasonable limit for email
	
	// HTTP client configuration
	MaxIdleConns        = 10
	IdleConnTimeout     = 30 * time.Second
	MaxIdleConnsPerHost = 10
	
	// Library information
	LibraryName    = "OpenTaco-Analytics"
	LibraryVersion = "1.0"
)

type Sink interface {
	Write(e Event) error
}

// Logger interface for structured logging
type Logger interface {
	Error(msg string, fields ...interface{})
}

// DefaultLogger provides a simple implementation
type DefaultLogger struct{}

func (l *DefaultLogger) Error(msg string, fields ...interface{}) {
	// Simple structured logging format
	if len(fields) > 0 {
		fmt.Printf("[ERROR] %s %v\n", msg, fields)
	} else {
		fmt.Printf("[ERROR] %s\n", msg)
	}
}

// Global logger instance
var logger Logger = &DefaultLogger{}

// SetLogger allows setting a custom logger
func SetLogger(l Logger) {
	logger = l
}

type Event struct {
	Level      Level                  `json:"level"`
	Event      string                 `json:"event"`                // Event name
	Timestamp  time.Time              `json:"timestamp"`
	Properties map[string]interface{} `json:"properties,omitempty"` // Rich properties for Segment
	UserID     string                 `json:"user_id,omitempty"`
	SessionID  string                 `json:"session_id,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`    // System context (system_id, user_email, etc.)
}

type Client struct {
	mu    sync.RWMutex
	level Level
	sinks []Sink
}

// Global singleton client
var globalClient *Client
var systemIDManager SystemIDManagerInterface
var once sync.Once

func New(level Level, sinks ...Sink) *Client {
	return &Client{level: level, sinks: append([]Sink(nil), sinks...)}
}


// createSinks creates the appropriate sinks based on the sink type
func createSinks(sink string) []Sink {
	var sinks []Sink
	switch sink {
	case "debug":
		sinks = append(sinks, &DebugSink{})
	default:
		sinks = append(sinks, NewProductionSink())
	}
	return sinks
}

// parseAnalyticsLevel parses the analytics level from environment
func parseAnalyticsLevel() Level {
	lvl, err := ParseLevel(os.Getenv("ANALYTICS_LEVEL"))
	if err != nil {
		return Default
	}
	return lvl
}

// InitGlobal initializes the global analytics client (call once from main)
func InitGlobal(sink string) {
	once.Do(func() {
		globalClient = New(parseAnalyticsLevel(), createSinks(sink)...)
	})
}

// InitGlobalWithSystemID initializes analytics with system ID management
func InitGlobalWithSystemID(sink string, store interface{}) {
	once.Do(func() {
		globalClient = New(parseAnalyticsLevel(), createSinks(sink)...)
		
		if store != nil {
			if s3Store, ok := store.(storage.S3Store); ok {
				systemIDManager = NewSystemIDManager(s3Store)
			} else {
				// Create a fallback system ID manager for non-S3 storage
				systemIDManager = NewFallbackSystemIDManager()
			}
		} else {
			// Create a fallback system ID manager when no storage is available
			systemIDManager = NewFallbackSystemIDManager()
		}
	})
}

// Package-level functions that use the global client
func Send(e Event) error {
	if globalClient == nil {
		return fmt.Errorf("analytics not initialized - call InitGlobal or InitGlobalWithSystemID first")
	}
	return globalClient.Send(e)
}

// sendEventWithLevel is a helper that creates and sends an event with the specified level
func sendEventWithLevel(level Level, data interface{}) error {
	// Input validation
	if data == nil {
		return fmt.Errorf("event data cannot be nil")
	}
	
	event := Event{
		Level:     level,
		Timestamp: time.Now(),
		Context:   getSystemContext(),
	}
	
	// Handle both string and complex object data
	switch v := data.(type) {
	case string:
		event.Event = v
	case map[string]interface{}:
		// If it's a map, treat it as properties and extract event name
		if eventName, ok := v["event"].(string); ok {
			event.Event = eventName
			event.Properties = v
		} else {
			// If no event name, use a default
			event.Event = "Custom Event"
			event.Properties = v
		}
	default:
		// For other types, convert to string for event name
		event.Event = fmt.Sprintf("%v", v)
	}
	
	return Send(event)
}

// SendEssential sends an essential-level event with automatic system context
func SendEssential(data interface{}) error {
	return sendEventWithLevel(Essential, data)
}

func SendReduced(data interface{}) error {
	return sendEventWithLevel(Reduced, data)
}

func SendDefault(data interface{}) error {
	return sendEventWithLevel(Default, data)
}

// getSystemContext returns the current system context (system_id, user_email, etc.)
func getSystemContext() map[string]interface{} {
	context := make(map[string]interface{})
	
	if systemIDManager != nil {
		// Always include system_id (should always be available)
		if systemID := systemIDManager.GetSystemID(); systemID != "" {
			context["system_id"] = systemID
		}
		// Always include user_email (empty string if not set)
		context["user_email"] = systemIDManager.GetUserEmail()
	}
	
	return context
}

// isValidEmail performs basic email format validation
func isValidEmail(email string) bool {
	// Basic email regex pattern
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}


// GetSystemID returns the current system ID (if available)
func GetSystemID() string {
	if systemIDManager == nil {
		return ""
	}
	return systemIDManager.GetSystemID()
}

// GetOrCreateSystemID retrieves or creates the system ID
func GetOrCreateSystemID(ctx context.Context) (string, error) {
	if systemIDManager == nil {
		return "", fmt.Errorf("system ID manager not initialized")
	}
	return systemIDManager.GetOrCreateSystemID(ctx)
}

// PreloadSystemID attempts to load the system ID without creating a new one
func PreloadSystemID(ctx context.Context) error {
	if systemIDManager == nil {
		return fmt.Errorf("system ID manager not initialized")
	}
	return systemIDManager.PreloadSystemID(ctx)
}

// IsSystemIDLoaded returns true if the system ID has been loaded
func IsSystemIDLoaded() bool {
	if systemIDManager == nil {
		return false
	}
	return systemIDManager.IsLoaded()
}


// SetUserEmail stores the user email
func SetUserEmail(ctx context.Context, email string) error {
	if systemIDManager == nil {
		return fmt.Errorf("system ID manager not initialized")
	}
	
	// Input validation
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}
	
	// Basic email format validation
	if !isValidEmail(email) {
		return fmt.Errorf("invalid email format: %s", email)
	}
	
	return systemIDManager.SetUserEmail(ctx, email)
}

// GetUserEmail returns the current user email
func GetUserEmail() string {
	if systemIDManager == nil {
		return ""
	}
	return systemIDManager.GetUserEmail()
}

func ParseLevel(s string) (Level, error) {
	switch s {
	case "off", "OFF":
		return Off, nil
	case "essential", "ESSENTIAL":
		return Essential, nil
	case "reduced", "REDUCED":
		return Reduced, nil
	case "default", "DEFAULT", "":
		return Default, nil
	default:
		// Try parsing as number
		if i, err := strconv.Atoi(s); err == nil && i >= 0 && i <= 3 {
			return Level(i), nil
		}
		return Default, errors.New("invalid level")
	}
}

func (c *Client) Send(e Event) error {
	c.mu.RLock()
	setLevel := c.level
	sinks := append([]Sink(nil), c.sinks...)
	c.mu.RUnlock()

	if e.Level > setLevel { // if event level is greater than configuration
		return nil // return nothing
	}

	var errs []error
	for _, s := range sinks {
		if err := s.Write(e); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}


// DebugSink writes to STDOUT
type DebugSink struct{}

func (d *DebugSink) Write(e Event) error {
	b, _ := json.Marshal(e)
	println("[DEBUG]", string(b))
	return nil
}

// ProductionSink sends events to a remote analytics endpoint
type ProductionSink struct {
	endpoint   string
	httpClient *http.Client
}

func NewProductionSink() *ProductionSink {
	return &ProductionSink{
		endpoint: DefaultAnalyticsEndpoint,
		httpClient: &http.Client{
			Timeout: DefaultRequestTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        MaxIdleConns,
				IdleConnTimeout:     IdleConnTimeout,
				DisableCompression:  true,
				MaxIdleConnsPerHost: MaxIdleConnsPerHost,
			},
		},
	}
}

func (p *ProductionSink) Write(e Event) error {
	// Send async to avoid blocking the main application
	// Use a buffered channel to prevent goroutine leaks
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), AsyncRequestTimeout)
		defer cancel()
		
		if err := p.sendEventWithContext(ctx, e); err != nil {
			// Use structured logging
			logger.Error("analytics: failed to send event", "error", err, "event", e.Event)
		}
	}()
	
	return nil // Always return nil to not block the application
}

func (p *ProductionSink) sendEvent(e Event) error {
	// Always use Segment format for OpenTaco analytics service
	return p.sendToSegment(e)
}

func (p *ProductionSink) sendEventWithContext(ctx context.Context, e Event) error {
	// Always use Segment format for OpenTaco analytics service
	return p.sendToSegmentWithContext(ctx, e)
}

// sendToSegmentWithContext formats the event with context for timeout control
func (p *ProductionSink) sendToSegmentWithContext(ctx context.Context, e Event) error {
	// Extract user ID from context if available
	userID := e.UserID
	if userID == "" && e.Context != nil {
		if uid, ok := e.Context["user_id"].(string); ok {
			userID = uid
		}
	}
	
	// Extract system ID and user email from context
	systemID := ""
	userEmail := ""
	if e.Context != nil {
		if sid, ok := e.Context["system_id"].(string); ok {
			systemID = sid
		}
		if email, ok := e.Context["user_email"].(string); ok {
			userEmail = email
		}
	}
	
	// Build Segment event
	segmentEvent := map[string]interface{}{
		"event":     e.Event,
		"timestamp": e.Timestamp,
		"properties": e.Properties,
		"context": map[string]interface{}{
			"system_id":  systemID,
			"user_email": userEmail,
			"library": map[string]interface{}{
				"name":    LibraryName,
				"version": LibraryVersion,
			},
		},
	}
	
	// Add user ID if available
	if userID != "" {
		segmentEvent["userId"] = userID
	}
	
	// Add session ID if available
	if e.SessionID != "" {
		segmentEvent["sessionId"] = e.SessionID
	}
	
	// Marshal the Segment event
	jsonData, err := json.Marshal(segmentEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal segment event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", LibraryName+"/"+LibraryVersion)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("analytics service returned status %d", resp.StatusCode)
	}

	return nil
}

// sendToSegment formats the event for OpenTaco analytics service (Segment-compatible format)
func (p *ProductionSink) sendToSegment(e Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultRequestTimeout)
	defer cancel()
	return p.sendToSegmentWithContext(ctx, e)
}
