package authz

import "testing"

func TestEnforcerAdminPoliciesMatchExactAndWildcardPaths(t *testing.T) {
	enforcer, err := NewEnforcer()
	if err != nil {
		t.Fatalf("NewEnforcer() error = %v", err)
	}

	tests := []struct {
		role     string
		resource string
		action   string
		want     bool
	}{
		{role: "admin", resource: "/admin", action: "GET", want: true},
		{role: "admin", resource: "/admin/users", action: "GET", want: true},
		{role: "admin", resource: "/api/v1/users", action: "GET", want: true},
		{role: "admin", resource: "/api/v1/users/123", action: "PATCH", want: true},
		{role: "user", resource: "/admin", action: "GET", want: false},
		{role: "user", resource: "/admin/users", action: "GET", want: false},
	}

	for _, tt := range tests {
		got, err := enforcer.Enforce(tt.role, tt.resource, tt.action)
		if err != nil {
			t.Fatalf("Enforce(%q, %q, %q) error = %v", tt.role, tt.resource, tt.action, err)
		}
		if got != tt.want {
			t.Fatalf("Enforce(%q, %q, %q) = %v, want %v", tt.role, tt.resource, tt.action, got, tt.want)
		}
	}
}
