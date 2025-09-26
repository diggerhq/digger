package deps

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"sort"
	"strings"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/storage"
	"github.com/mr-tron/base58"
)

const SystemGraphUnitID = "__opentaco_system"

// TFState models the minimal Terraform 1.x state structure we need
type TFState struct {
	Serial    int          `json:"serial"`
	Lineage   string       `json:"lineage"`
	Resources []TFResource `json:"resources"`
}

type TFResource struct {
	Mode      string       `json:"mode"`
	Type      string       `json:"type"`
	Name      string       `json:"name"`
	Provider  string       `json:"provider"`
	Instances []TFInstance `json:"instances"`
}

type TFInstance struct {
	Attributes map[string]interface{} `json:"attributes"`
}

// TFOutputs is a small view of TF state outputs
type TFOutputs struct {
	Outputs map[string]struct {
		Value interface{} `json:"value"`
	} `json:"outputs"`
}

// UpdateGraphOnWrite updates dependency edges in the graph tfstate in response to a write
// to unitID with content newTFState. It performs both outgoing (source refresh) and incoming
// (target acknowledge) updates in a single locked read-modify-write cycle.
func UpdateGraphOnWrite(ctx context.Context, store storage.UnitStore, unitID string, newTFState []byte) {
	// Fast exits: graph unit must exist and be lockable. Never fail the caller's write.
	// Acquire lock
	fmt.Println("updating graph on write")
	lock := &storage.LockInfo{ID: fmt.Sprintf("deps-%d", time.Now().UnixNano()), Who: "opentaco-deps", Version: "1.0.0", Created: time.Now()}
	if err := store.Lock(ctx, SystemGraphUnitID, lock); err != nil {
		fmt.Println("graph already locked by someone %v", err)
		// Graph missing or locked by someone else — skip quietly
		return
	}
	defer func() { _ = store.Unlock(ctx, SystemGraphUnitID, lock.ID) }()

	// Read current graph tfstate
	graphBytes, err := store.Download(ctx, SystemGraphUnitID)
	if err != nil || len(graphBytes) == 0 {
		fmt.Println("error reading graph bytes %v\n\n\n", err)
		return
	}

	var graph TFState
	if err := json.Unmarshal(graphBytes, &graph); err != nil {
		fmt.Println("error unmarshalling graph %v\n\n\n", err)
		return
	}

	// Parse new tfstate outputs once
	var outs TFOutputs
	_ = json.Unmarshal(newTFState, &outs) // if this fails, outs.Outputs stays nil

	normID := normalizeUnitID(unitID)

	now := time.Now().UTC().Format(time.RFC3339)

	fmt.Println("figuring out changed objects ..")
	changed := false
	for ri := range graph.Resources {
		r := &graph.Resources[ri]
		if r.Type != "opentaco_dependency" {
			continue
		}
		for ii := range r.Instances {
			inst := &r.Instances[ii]
			attrs := inst.Attributes
			if attrs == nil {
				continue
			}

			fromID := normalizeUnitID(getString(attrs["from_unit_id"]))
			toID := normalizeUnitID(getString(attrs["to_unit_id"]))
			fromOut := getString(attrs["from_output"])

			// Ensure fields exist to avoid nil panics when writing back
			_ = getString(attrs["in_digest"])    // probe
			_ = getString(attrs["out_digest"])   // probe
			status := getString(attrs["status"]) // may be empty

			// A) Outgoing (source refresh)
			if fromID == normID {
				// If from_output exists in new state outputs, recompute in_digest and status
				if outs.Outputs != nil {
					if ov, ok := outs.Outputs[fromOut]; ok {
						dig := digestValue(ov.Value)
						if getString(attrs["in_digest"]) != dig {
							attrs["in_digest"] = dig
							attrs["last_in_at"] = now
							changed = true
						}
						// Recompute status relative to out_digest
						outD := getString(attrs["out_digest"])
						newStatus := "pending"
						if dig != "" && outD != "" && dig == outD {
							newStatus = "ok"
						}
						if status != newStatus {
							attrs["status"] = newStatus
							changed = true
						}
					} else {
						// Source output missing
						if status != "unknown" {
							attrs["status"] = "unknown"
							changed = true
						}
					}
				} else {
					// Cannot parse outputs, set unknown
					if status != "unknown" {
						attrs["status"] = "unknown"
						changed = true
					}
				}
			}

			// B) Incoming (target acknowledge)
			if toID == normID {
				inD := getString(attrs["in_digest"])
				if inD != "" {
					if getString(attrs["out_digest"]) != inD {
						attrs["out_digest"] = inD
						attrs["last_out_at"] = now
						attrs["status"] = "ok"
						changed = true
					} else if status != "ok" {
						attrs["status"] = "ok"
						changed = true
					}
				}
			}
		}
	}

	fmt.Println("completed the graph traversal")

	if !changed {
		return
	}

	// Bump serial and write back
	graph.Serial++
	updated, err := json.Marshal(&graph)
	if err != nil {
		fmt.Println("error during marshalling updated graph %v", updated)
		return
	}

	spew.Dump(updated)
	// Pass lockID to satisfy write while locked
	_ = store.Upload(ctx, SystemGraphUnitID, updated, lock.ID)
}

// ComputeUnitStatus reads the graph tfstate and returns the status payload for a given unitID.
// If the graph is missing/corrupt, it returns a best-effort empty green status.
func ComputeUnitStatus(ctx context.Context, store storage.UnitStore, unitID string) (*UnitStatus, error) {
	b, err := store.Download(ctx, SystemGraphUnitID)
	if err != nil || len(b) == 0 {
		// Treat missing graph as no edges
		return &UnitStatus{StateID: unitID, Status: "green", Incoming: nil, Summary: Summary{}}, nil
	}
	var st TFState
	if err := json.Unmarshal(b, &st); err != nil {
		return &UnitStatus{StateID: unitID, Status: "green", Incoming: nil, Summary: Summary{}}, nil
	}

	normTarget := normalizeUnitID(unitID)

	// Collect edges and build adjacency
	type edge struct {
		EdgeID     string
		FromUnit   string
		FromOutput string
		ToUnit     string
		InDigest   string
		OutDigest  string
		Status     string
		LastInAt   string
		LastOutAt  string
	}

	var edges []edge
	adj := map[string][]string{}
	incoming := []IncomingEdge{}
	for _, r := range st.Resources {
		if r.Type != "opentaco_dependency" {
			continue
		}
		for _, inst := range r.Instances {
			a := inst.Attributes
			if a == nil {
				continue
			}
			frm := normalizeUnitID(getString(a["from_unit_id"]))
			to := normalizeUnitID(getString(a["to_unit_id"]))
			e := edge{
				EdgeID:     getString(a["id"]), // not always present; provider sets ID in state attributes automatically
				FromUnit:   frm,
				FromOutput: getString(a["from_output"]),
				ToUnit:     to,
				InDigest:   getString(a["in_digest"]),
				OutDigest:  getString(a["out_digest"]),
				Status:     getString(a["status"]),
				LastInAt:   getString(a["last_in_at"]),
				LastOutAt:  getString(a["last_out_at"]),
			}
			edges = append(edges, e)
			adj[frm] = append(adj[frm], to)

			if to == normTarget {
				incoming = append(incoming, IncomingEdge{
					EdgeID:     e.EdgeID,
					FromUnitID: e.FromUnit,
					FromOutput: e.FromOutput,
					Status:     e.Status,
					InDigest:   e.InDigest,
					OutDigest:  e.OutDigest,
					LastInAt:   e.LastInAt,
					LastOutAt:  e.LastOutAt,
				})
			}
		}
	}

	// Determine red units: any unit with an incoming edge pending.
	red := map[string]bool{}
	for _, e := range edges {
		if e.Status == "pending" {
			red[e.ToUnit] = true
		}
	}

	// Propagate yellow from red upstreams
	yellow := map[string]bool{}
	// BFS from all red units
	q := make([]string, 0, len(red))
	seen := map[string]bool{}
	for s := range red {
		q = append(q, s)
		seen[s] = true
	}
	for len(q) > 0 {
		cur := q[0]
		q = q[1:]
		for _, nxt := range adj[cur] {
			if seen[nxt] {
				continue
			}
			yellow[nxt] = true
			seen[nxt] = true
			q = append(q, nxt)
		}
	}

	// Compute incoming summary for target
	sum := Summary{}
	for _, ie := range incoming {
		switch ie.Status {
		case "ok":
			sum.IncomingOK++
		case "pending":
			sum.IncomingPending++
		default:
			sum.IncomingUnknown++
		}
	}

	// Determine target status
	stStatus := "green"
	if red[normTarget] {
		stStatus = "red"
	} else if yellow[normTarget] {
		stStatus = "yellow"
	}

	// Sort incoming edges by status for stable output
	sort.Slice(incoming, func(i, j int) bool {
		if incoming[i].Status == incoming[j].Status {
			return incoming[i].FromUnitID < incoming[j].FromUnitID
		}
		// pending first, then unknown, then ok
		order := map[string]int{"pending": 0, "unknown": 1, "ok": 2}
		return order[incoming[i].Status] < order[incoming[j].Status]
	})

	return &UnitStatus{StateID: unitID, Status: stStatus, Incoming: incoming, Summary: sum}, nil
}

// Types for API response
type UnitStatus struct {
	StateID  string         `json:"unit_id"`
	Status   string         `json:"status"`
	Incoming []IncomingEdge `json:"incoming"`
	Summary  Summary        `json:"summary"`
}

type IncomingEdge struct {
	EdgeID     string `json:"edge_id,omitempty"`
	FromUnitID string `json:"from_unit_id"`
	FromOutput string `json:"from_output"`
	Status     string `json:"status"`
	InDigest   string `json:"in_digest,omitempty"`
	OutDigest  string `json:"out_digest,omitempty"`
	LastInAt   string `json:"last_in_at,omitempty"`
	LastOutAt  string `json:"last_out_at,omitempty"`
}

type Summary struct {
	IncomingOK      int `json:"incoming_ok"`
	IncomingPending int `json:"incoming_pending"`
	IncomingUnknown int `json:"incoming_unknown"`
}

// Helpers

func normalizeUnitID(id string) string {
	s := strings.TrimSpace(id)
	s = strings.Trim(s, "/")
	// Accept either with or without terraform.tfstate suffix — normalize by trimming it
	s = strings.TrimSuffix(s, "/terraform.tfstate")
	return s
}

func getString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case json.Number:
		return t.String()
	case nil:
		return ""
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

// digestValue computes SHA-256 over canonical JSON bytes and returns base58 string
func digestValue(v interface{}) string {
	b := canonicalJSON(v)
	if b == nil {
		return ""
	}
	h := sha256.Sum256(b)
	return base58.Encode(h[:])
}

// canonicalJSON produces a deterministic JSON encoding for a limited set of values
func canonicalJSON(v interface{}) []byte {
	// Handle common Terraform output shapes: nil, bool, float64, string, []any, map[string]any
	switch t := v.(type) {
	case nil:
		return []byte("null")
	case bool:
		if t {
			return []byte("true")
		}
		return []byte("false")
	case float64:
		// Use json.Marshal for numbers which is stable for float64
		b, _ := json.Marshal(t)
		return b
	case int, int64, uint64, json.Number:
		b, _ := json.Marshal(t)
		return b
	case string:
		b, _ := json.Marshal(t)
		return b
	case []interface{}:
		// Arrays in order
		var out []byte
		out = append(out, '[')
		for i, el := range t {
			if i > 0 {
				out = append(out, ',')
			}
			out = append(out, canonicalJSON(el)...)
		}
		out = append(out, ']')
		return out
	case map[string]interface{}:
		// Sort keys lexicographically
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var out []byte
		out = append(out, '{')
		for i, k := range keys {
			if i > 0 {
				out = append(out, ',')
			}
			kb, _ := json.Marshal(k)
			out = append(out, kb...)
			out = append(out, ':')
			out = append(out, canonicalJSON(t[k])...)
		}
		out = append(out, '}')
		return out
	default:
		// Attempt to coerce via JSON first
		var m map[string]interface{}
		if b, err := json.Marshal(t); err == nil {
			if err := json.Unmarshal(b, &m); err == nil {
				return canonicalJSON(m)
			}
		}
		// As a last resort, encode via json.Marshal
		b, _ := json.Marshal(t)
		return b
	}
}

// Merge two error values, preferring the non-nil one. Not used now but may help later.
func mergeErr(a, b error) error {
	if a != nil {
		return a
	}
	return b
}
