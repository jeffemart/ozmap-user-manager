package models

import (
	"encoding/json"
	"testing"
)

func TestUserResultJSONFieldNames(t *testing.T) {
	data, err := json.Marshal(UserResult{
		Email:    "person@example.com",
		Username: "person@example.com",
		RoleName: "operator",
		Status:   "created",
		Message:  "Usuário criado com sucesso",
	})
	if err != nil {
		t.Fatalf("marshal UserResult: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal UserResult: %v", err)
	}
	for _, key := range []string{"email", "username", "roleName", "status", "message"} {
		if _, ok := result[key]; !ok {
			t.Errorf("JSON response missing field %q: %s", key, data)
		}
	}
}
