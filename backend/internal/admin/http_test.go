package admin

import (
	"testing"

	"github.com/venomimonstro/chat-hive/backend/internal/identity"
)

func TestRequiresPasskeyStepUp(t *testing.T) {
	tests := []struct {
		name      string
		roles     []string
		method    string
		requires  bool
	}{
		{name: "owner email", roles: []string{"owner"}, method: "email", requires: true},
		{name: "security yandex", roles: []string{"security"}, method: "yandex", requires: true},
		{name: "owner passkey", roles: []string{"owner"}, method: "passkey", requires: false},
		{name: "moderator email", roles: []string{"moderator"}, method: "email", requires: false},
		{name: "system admin email", roles: []string{"system_admin"}, method: "email", requires: false},
		{name: "mixed owner role", roles: []string{"moderator","owner"}, method: "legacy", requires: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := requiresPasskeyStepUp(
				Principal{UserID: "00000000-0000-0000-0000-000000000001", Roles: tc.roles},
				identity.AuthenticatedSession{AuthMethod: tc.method},
			)
			if got != tc.requires {
				t.Fatalf("requiresPasskeyStepUp()=%v want %v", got, tc.requires)
			}
		})
	}
}
