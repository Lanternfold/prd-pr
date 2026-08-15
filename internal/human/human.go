package human

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	SchemaVersion = 1
	RequestRel    = ".project/human/request.json"
	ResponseRel   = ".project/human/response.json"
	CredMetaRel   = ".project/human/credentials.json"

	UrgencyLow    = "low"
	UrgencyNormal = "normal"
	UrgencyHigh   = "high"

	KindCredential   = "credential"
	KindAmbiguous    = "ambiguous_requirement"
	KindArchitecture = "architecture_decision"
	KindRepairFail   = "repeated_repair_failure"
	KindManualAC     = "manual_acceptance"
	KindUnsafe       = "unsafe_operation"
	KindBlocked      = "blocked_dependency"
	KindUncertainty  = "model_uncertainty"
	KindBudget       = "cost_budget"
)

const (
	CredMissing           = "MISSING"
	CredPresentUnverified = "PRESENT_UNVERIFIED"
	CredPresentVerified   = "PRESENT_VERIFIED"
	StatusConfirmed       = "CONFIRMED"
)

// Request is one human intervention. Ask one thing at a time.
type Request struct {
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`
	Reason        string `json:"reason"`
	Kind          string `json:"kind"`
	Phase         string `json:"phase,omitempty"`
	Task          string `json:"task,omitempty"`
	Attempted     string `json:"what_was_attempted,omitempty"`
	Needed        string `json:"what_is_needed"`
	EstimatedTime string `json:"estimated_time,omitempty"`
	Urgency       string `json:"urgency"`
	Deadline      string `json:"deadline,omitempty"`
	Optional      bool   `json:"optional"`
	Credential    string `json:"credential_name,omitempty"`
}

// Response is the human's answer. Secret values must not appear here.
type Response struct {
	RequestID string `json:"request_id"`
	Text      string `json:"text,omitempty"`
	Status    string `json:"status,omitempty"`
	At        string `json:"at"`
}

// CredentialMeta records presence only.
type CredentialMeta struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type Forecast struct {
	ExpectedCount   int      `json:"expected_count"`
	ExpectedReasons []string `json:"expected_reasons,omitempty"`
	ExpectedMinutes float64  `json:"expected_human_minutes"`
}

func WriteRequest(root string, req Request) error {
	if req.SchemaVersion == 0 {
		req.SchemaVersion = SchemaVersion
	}
	if req.ID == "" {
		req.ID = fmt.Sprintf("h_%d", time.Now().UnixNano())
	}
	if req.Urgency == "" {
		req.Urgency = UrgencyNormal
	}
	return writeJSON(root, RequestRel, req)
}

func LoadRequest(root string) (Request, error) {
	var req Request
	err := readJSON(root, RequestRel, &req)
	return req, err
}

func WriteResponse(root string, res Response) error {
	if res.At == "" {
		res.At = time.Now().UTC().Format(time.RFC3339Nano)
	}
	return writeJSON(root, ResponseRel, res)
}

func LoadResponse(root string) (Response, error) {
	var res Response
	err := readJSON(root, ResponseRel, &res)
	return res, err
}

func RecordCredential(root string, name, status string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("credential name is required")
	}
	var list []CredentialMeta
	_ = readJSON(root, CredMetaRel, &list)
	found := false
	for i := range list {
		if list[i].Name == name {
			list[i].Status = status
			found = true
			break
		}
	}
	if !found {
		list = append(list, CredentialMeta{Name: name, Status: status})
	}
	return writeJSON(root, CredMetaRel, list)
}

func NextMissingCredential(names []string, root string) string {
	var list []CredentialMeta
	_ = readJSON(root, CredMetaRel, &list)
	have := map[string]string{}
	for _, c := range list {
		have[c.Name] = c.Status
	}
	for _, n := range names {
		if have[n] == "" || have[n] == CredMissing {
			return n
		}
	}
	return ""
}

func writeJSON(root, rel string, v any) error {
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func readJSON(root, rel string, v any) error {
	path := filepath.Join(root, filepath.FromSlash(rel))
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

// ManualConfirmed reports whether the human explicitly confirmed pending manual acceptance.
func ManualConfirmed(root string) bool {
	req, err := LoadRequest(root)
	if err != nil || req.Kind != KindManualAC {
		return false
	}
	res, err := LoadResponse(root)
	if err != nil {
		return false
	}
	if req.ID != "" && res.RequestID != "" && req.ID != res.RequestID {
		return false
	}
	switch res.Status {
	case StatusConfirmed, CredPresentVerified:
		return true
	default:
		return false
	}
}
