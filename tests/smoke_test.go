package tests

import (
	"github.com/example/ozmap-user-manager/internal/security"
	"testing"
)

func TestSHA256(t *testing.T) {
	if security.SHA256Hex("password") != "5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8" {
		t.Fatal("unexpected hash")
	}
}
