package analytics

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/diggerhq/digger/opentaco/internal/storage"
)

// SystemIDManager handles anonymous system identification using UnitStore.
// System ID and user email are stored as units with special IDs.
type SystemIDManager struct {
	store     storage.UnitStore
	systemID  string
	userEmail string
	loaded    bool
	mu        sync.RWMutex
}

// SystemIDKey is the unit ID where we store the system identifier
const SystemIDKey = "__system_id"
const UserEmailKey = "__user_email"

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
func NewSystemIDManager(store storage.UnitStore) *SystemIDManager {
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

// readSystemID reads the system ID from storage
func (m *SystemIDManager) readSystemID(ctx context.Context) (string, error) {
	data, err := m.store.Download(ctx, SystemIDKey)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// writeSystemID writes the system ID to storage
func (m *SystemIDManager) writeSystemID(ctx context.Context, systemID string) error {
	return m.store.Upload(ctx, SystemIDKey, []byte(systemID), "")
}

// readUserEmail reads the user email from storage
func (m *SystemIDManager) readUserEmail(ctx context.Context) (string, error) {
	data, err := m.store.Download(ctx, UserEmailKey)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// writeUserEmail writes the user email to storage
func (m *SystemIDManager) writeUserEmail(ctx context.Context, email string) error {
	return m.store.Upload(ctx, UserEmailKey, []byte(email), "")
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

