package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_CreateUnit(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/units" || r.Method != "POST" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		var req CreateUnitRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := CreateUnitResponse{
			ID:      req.Name,
			Created: time.Now(),
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Test client
	client := NewClient(server.URL)
	resp, err := client.CreateUnit(context.Background(), "test/unit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.ID != "test/unit" {
		t.Errorf("expected ID 'test/unit', got %s", resp.ID)
	}
}

func TestClient_ListUnits(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/units" || r.Method != "GET" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("page") != "2" || r.URL.Query().Get("page_size") != "25" {
			t.Errorf("unexpected pagination params: %v", r.URL.Query())
		}

		resp := ListUnitsResponse{
			Units: []*UnitMetadata{
				{
					ID:      "unit1",
					Size:    100,
					Updated: time.Now(),
					Locked:  false,
				},
				{
					ID:      "unit2",
					Size:    200,
					Updated: time.Now(),
					Locked:  true,
				},
			},
			Count:    2,
			Total:    5,
			Page:     2,
			PageSize: 25,
		}

		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Test client
	client := NewClient(server.URL)
	resp, err := client.ListUnits(context.Background(), "", 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Count != 2 {
		t.Errorf("expected 2 units, got %d", resp.Count)
	}

	if len(resp.Units) != 2 {
		t.Errorf("expected 2 units in array, got %d", len(resp.Units))
	}

	if resp.Total != 5 || resp.Page != 2 || resp.PageSize != 25 {
		t.Errorf("unexpected pagination metadata: %+v", resp)
	}
}

func TestClient_ErrorHandling(t *testing.T) {
	// Mock server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unit already exists"})
	}))
	defer server.Close()

	// Test client
	client := NewClient(server.URL)
	_, err := client.CreateUnit(context.Background(), "test/unit")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err.Error() != "HTTP 409: Unit already exists" {
		t.Errorf("unexpected error message: %v", err)
	}
}
