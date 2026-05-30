package authz

import (
	"embed"
	"fmt"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	stringadapter "github.com/casbin/casbin/v2/persist/string-adapter"
)

//go:embed model.conf
var modelFS embed.FS

// defaultPolicies defines the initial RBAC policy rules.
// Format: p, role, resource_pattern, http_method_or_wildcard
var defaultPolicies = [][]string{
	// Admin can access everything.
	{"admin", "*", "*"},
	// Regular users: auth endpoints.
	{"user", "/api/v1/auth/*", "*"},
	// Regular users: billing.
	{"user", "/api/v1/billing/*", "*"},
	// Regular users: own profile.
	{"user", "/api/v1/users/me", "*"},
	// Admin: user management.
	{"admin", "/api/v1/users", "*"},
	{"admin", "/api/v1/users/*", "*"},
	// Admin: admin pages.
	{"admin", "/admin", "*"},
	{"admin", "/admin/*", "*"},
}

// Enforcer wraps a casbin.Enforcer to provide a clean authorization API.
type Enforcer struct {
	enforcer *casbin.Enforcer
}

// NewEnforcer creates a new Enforcer with the embedded RBAC model and
// default in-memory policies. For production use with persistent policies,
// replace the string adapter with a database-backed adapter.
func NewEnforcer() (*Enforcer, error) {
	modelData, err := modelFS.ReadFile("model.conf")
	if err != nil {
		return nil, fmt.Errorf("authz: reading embedded model: %w", err)
	}

	m, err := model.NewModelFromString(string(modelData))
	if err != nil {
		return nil, fmt.Errorf("authz: parsing model: %w", err)
	}

	// Use a string adapter for in-memory policy storage.
	// In production, swap this with a pgx/sqlx adapter for persistence.
	sa := stringadapter.NewAdapter("# policy")

	e, err := casbin.NewEnforcer(m, sa)
	if err != nil {
		return nil, fmt.Errorf("authz: creating enforcer: %w", err)
	}

	// Load default policies.
	for _, p := range defaultPolicies {
		if _, err := e.AddPolicy(p[0], p[1], p[2]); err != nil {
			return nil, fmt.Errorf("authz: adding default policy %v: %w", p, err)
		}
	}

	return &Enforcer{enforcer: e}, nil
}

// Enforce checks whether a role is allowed to perform an action on a resource.
// Returns true if the request is permitted by the loaded policies.
func (e *Enforcer) Enforce(role, resource, action string) (bool, error) {
	allowed, err := e.enforcer.Enforce(role, resource, action)
	if err != nil {
		return false, fmt.Errorf("authz: enforcing policy: %w", err)
	}
	return allowed, nil
}

// AddPolicy adds a new policy rule granting role access to resource for action.
func (e *Enforcer) AddPolicy(role, resource, action string) error {
	_, err := e.enforcer.AddPolicy(role, resource, action)
	if err != nil {
		return fmt.Errorf("authz: adding policy: %w", err)
	}
	return nil
}

// RemovePolicy removes a policy rule.
func (e *Enforcer) RemovePolicy(role, resource, action string) error {
	_, err := e.enforcer.RemovePolicy(role, resource, action)
	if err != nil {
		return fmt.Errorf("authz: removing policy: %w", err)
	}
	return nil
}

// GetPoliciesForRole returns all policy rules associated with the given role.
// Each returned slice has the format [role, resource, action].
func (e *Enforcer) GetPoliciesForRole(role string) [][]string {
	policies, _ := e.enforcer.GetFilteredPolicy(0, role)
	return policies
}
