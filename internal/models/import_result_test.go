package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestImportResultJSONFieldNames(t *testing.T) {
	message := "Role inválida"
	data, err := json.Marshal(ImportResult{
		ID:           1,
		ImportID:     "import-1",
		Email:        "user@example.com",
		RoleName:     "operator",
		Status:       "error",
		ErrorMessage: &message,
		CreatedAt:    time.Date(2026, time.October, 5, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("marshal ImportResult: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal ImportResult: %v", err)
	}
	for _, key := range []string{"email", "role_name", "status", "error_message", "created_at"} {
		if _, ok := result[key]; !ok {
			t.Errorf("JSON response missing field %q: %s", key, data)
		}
	}
}
