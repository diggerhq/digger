package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_CreateState(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/states" || r.Method != "POST" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		var req CreateStateRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := CreateStateResponse{
			ID:      req.ID,
			Created: time.Now(),
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Test client
	client := NewClient(server.URL)
	resp, err := client.CreateState(context.Background(), "test/state")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.ID != "test/state" {
		t.Errorf("expected ID 'test/state', got %s", resp.ID)
	}
}

func TestClient_ListStates(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/states" || r.Method != "GET" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		resp := ListStatesResponse{
			States: []*StateMetadata{
				{
					ID:      "state1",
					Exists:  true,
					Size:    100,
					Updated: time.Now(),
					Locked:  false,
				},
				{
					ID:      "state2",
					Exists:  true,
					Size:    200,
					Updated: time.Now(),
					Locked:  true,
				},
			},
			Count: 2,
		}

		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Test client
	client := NewClient(server.URL)
	resp, err := client.ListStates(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Count != 2 {
		t.Errorf("expected 2 states, got %d", resp.Count)
	}

	if len(resp.States) != 2 {
		t.Errorf("expected 2 states in array, got %d", len(resp.States))
	}
}

func TestClient_ErrorHandling(t *testing.T) {
	// Mock server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "State already exists",
		})
	}))
	defer server.Close()

	// Test client
	client := NewClient(server.URL)
	_, err := client.CreateState(context.Background(), "test/state")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err.Error() != "HTTP 409: State already exists" {
		t.Errorf("unexpected error message: %v", err)
	}
}