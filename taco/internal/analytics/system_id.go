package analytics

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/diggerhq/digger/opentaco/internal/storage"
)

// SystemIDManager handles anonymous system identification via S3
type SystemIDManager struct {
	store     storage.S3Store
	systemID  string
	userEmail string
	loaded    bool
	mu        sync.RWMutex
}

// SystemIDKey is the S3 key where we store the system identifier
const SystemIDKey = "system_id/id"
const UserEmailKey = "system_id/email"

// SystemIDManagerInterface defines the interface for system ID management
type SystemIDManagerInterface interface {
	GetOrCreateSystemID(ctx context.Context) (string, error)
	GetSystemID() string
	IsLoaded() bool
	PreloadSystemID(ctx context.Context) error
	SetUserEmail(ctx context.Context, email string) error
	GetUserEmail() string
}

// NewSystemIDManager creates a new system ID manager
func NewSystemIDManager(store storage.S3Store) *SystemIDManager {
	return &SystemIDManager{
		store: store,
	}
}

// GetOrCreateSystemID retrieves the existing system ID or creates a new one
func (m *SystemIDManager) GetOrCreateSystemID(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.loaded {
		return m.systemID, nil
	}

	// Try to read existing system ID from S3
	existingID, err := m.readSystemID(ctx)
	if err == nil && existingID != "" {
		m.systemID = existingID
		m.loaded = true
		return m.systemID, nil
	}

	// Create new system ID
	newID, err := m.generateSystemID()
	if err != nil {
		return "", fmt.Errorf("failed to generate system ID: %w", err)
	}

	// Try to write the new system ID (with optimistic concurrency)
	err = m.writeSystemID(ctx, newID)
	if err != nil {
		// If write failed, try to read again (another instance might have created it)
		existingID, readErr := m.readSystemID(ctx)
		if readErr == nil && existingID != "" {
			m.systemID = existingID
			m.loaded = true
			return m.systemID, nil
		}
		return "", fmt.Errorf("failed to create system ID: %w", err)
	}

	m.systemID = newID
	m.loaded = true
	return m.systemID, nil
}

// GetSystemID returns the current system ID (without creating if missing)
func (m *SystemIDManager) GetSystemID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.systemID
}

// IsLoaded returns true if the system ID has been loaded
func (m *SystemIDManager) IsLoaded() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.loaded
}

// PreloadSystemID attempts to load the system ID without creating a new one
func (m *SystemIDManager) PreloadSystemID(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.loaded {
		return nil
	}

	existingID, err := m.readSystemID(ctx)
	if err == nil && existingID != "" {
		m.systemID = existingID
		m.loaded = true
		// Also try to load user email
		if email, err := m.readUserEmail(ctx); err == nil {
			m.userEmail = email
		}
		return nil
	}

	return fmt.Errorf("system ID not found in S3")
}

// SetUserEmail stores the user email in S3
func (m *SystemIDManager) SetUserEmail(ctx context.Context, email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	err := m.writeUserEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("failed to write user email: %w", err)
	}

	m.userEmail = email
	return nil
}

// GetUserEmail returns the current user email
func (m *SystemIDManager) GetUserEmail() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.userEmail
}

// readSystemID reads the system ID from S3
func (m *SystemIDManager) readSystemID(ctx context.Context) (string, error) {
	client := m.store.GetS3Client()
	bucket := m.store.GetS3Bucket()
	key := m.store.Key(SystemIDKey)

	result, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", err
	}
	defer result.Body.Close()

	// Read the system ID (should be a simple hex string)
	buf := make([]byte, SystemIDBufferSize) // 16 bytes = 32 hex chars
	n, err := result.Body.Read(buf)
	if err != nil && n == 0 {
		return "", err
	}

	return string(buf[:n]), nil
}

// writeSystemID writes the system ID to S3
func (m *SystemIDManager) writeSystemID(ctx context.Context, systemID string) error {
	client := m.store.GetS3Client()
	bucket := m.store.GetS3Bucket()
	key := m.store.Key(SystemIDKey)

	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        strings.NewReader(systemID),
		ContentType: aws.String("text/plain"),
		Metadata: map[string]string{
			"opentaco-system-id": "true",
			"created-at":         time.Now().UTC().Format(time.RFC3339),
		},
	})

	return err
}

// readUserEmail reads the user email from S3
func (m *SystemIDManager) readUserEmail(ctx context.Context) (string, error) {
	client := m.store.GetS3Client()
	bucket := m.store.GetS3Bucket()
	key := m.store.Key(UserEmailKey)

	result, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", err
	}
	defer result.Body.Close()

	// Read the user email
	buf := make([]byte, EmailBufferSize) // Reasonable limit for email
	n, err := result.Body.Read(buf)
	if err != nil && n == 0 {
		return "", err
	}

	return string(buf[:n]), nil
}

// writeUserEmail writes the user email to S3
func (m *SystemIDManager) writeUserEmail(ctx context.Context, email string) error {
	client := m.store.GetS3Client()
	bucket := m.store.GetS3Bucket()
	key := m.store.Key(UserEmailKey)

	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        strings.NewReader(email),
		ContentType: aws.String("text/plain"),
		Metadata: map[string]string{
			"opentaco-user-email": "true",
			"created-at":          time.Now().UTC().Format(time.RFC3339),
		},
	})

	return err
}

// generateSystemID creates a new anonymous system identifier
func (m *SystemIDManager) generateSystemID() (string, error) {
	// Generate 16 random bytes (128 bits)
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	// Convert to hex string (32 characters)
	return hex.EncodeToString(bytes), nil
}

// IsSystemIDValid checks if a system ID has the correct format
func IsSystemIDValid(systemID string) bool {
	if len(systemID) != 32 {
		return false
	}
	
	// Check if it's valid hex
	_, err := hex.DecodeString(systemID)
	return err == nil
}

// FallbackSystemIDManager handles system ID without S3 persistence
type FallbackSystemIDManager struct {
	systemID  string
	userEmail string
	loaded    bool
	mu        sync.RWMutex
}

// NewFallbackSystemIDManager creates a fallback system ID manager
func NewFallbackSystemIDManager() *FallbackSystemIDManager {
	return &FallbackSystemIDManager{}
}

// GetOrCreateSystemID creates a new system ID (no persistence)
func (m *FallbackSystemIDManager) GetOrCreateSystemID(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.loaded {
		return m.systemID, nil
	}

	// Generate new system ID
	newID, err := m.generateSystemID()
	if err != nil {
		return "", fmt.Errorf("failed to generate system ID: %w", err)
	}

	m.systemID = newID
	m.loaded = true
	return m.systemID, nil
}

// GetSystemID returns the current system ID
func (m *FallbackSystemIDManager) GetSystemID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.systemID
}

// IsLoaded returns true if the system ID has been loaded
func (m *FallbackSystemIDManager) IsLoaded() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.loaded
}

// PreloadSystemID is a no-op for fallback (always creates new)
func (m *FallbackSystemIDManager) PreloadSystemID(ctx context.Context) error {
	_, err := m.GetOrCreateSystemID(ctx)
	return err
}

// SetUserEmail stores the user email in memory (no persistence)
func (m *FallbackSystemIDManager) SetUserEmail(ctx context.Context, email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.userEmail = email
	return nil
}

// GetUserEmail returns the current user email
func (m *FallbackSystemIDManager) GetUserEmail() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.userEmail
}

// generateSystemID creates a new anonymous system identifier
func (m *FallbackSystemIDManager) generateSystemID() (string, error) {
	// Generate 16 random bytes (128 bits)
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	// Convert to hex string (32 characters)
	return hex.EncodeToString(bytes), nil
}
