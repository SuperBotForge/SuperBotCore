package authz

import (
	"testing"
)

// makeEnv builds an exprEnv with the given roles and user map for testing.
// No SpiceDB client is involved; Check and IsMember always return false.
func makeEnv(roles []string, userMap map[string]any) exprEnv {
	roleSet := make(map[string]bool, len(roles))
	for _, r := range roles {
		roleSet[r] = true
	}

	return exprEnv{
		User: userMap,
		Check: func(_, _, _ string) (bool, error) {
			return false, nil
		},
		IsMember: func(_, _ string) (bool, error) {
			return false, nil
		},
		HasRole: func(roleName string) bool {
			return roleSet[roleName]
		},
		HasAnyRole: func(roleNames ...string) bool {
			for _, rn := range roleNames {
				if roleSet[rn] {
					return true
				}
			}
			return false
		},
	}
}

func TestEvaluate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		expression string
		roles      []string
		userMap    map[string]any
		want       bool
		wantErr    bool
		errContain string
	}{
		{
			name:       "simple true",
			expression: "true",
			want:       true,
		},
		{
			name:       "simple false",
			expression: "false",
			want:       false,
		},
		{
			name:       "has_role matches",
			expression: `has_role("admin")`,
			roles:      []string{"admin"},
			want:       true,
		},
		{
			name:       "has_role missing",
			expression: `has_role("admin")`,
			roles:      []string{},
			want:       false,
		},
		{
			name:       "has_any_role matches one",
			expression: `has_any_role("admin", "mod")`,
			roles:      []string{"mod"},
			want:       true,
		},
		{
			name:       "has_any_role none match",
			expression: `has_any_role("admin")`,
			roles:      []string{"user"},
			want:       false,
		},
		{
			name:       "user attribute locale",
			expression: `user.locale == "ru"`,
			userMap:    map[string]any{"locale": "ru"},
			want:       true,
		},
		{
			name:       "complex OR expression",
			expression: `has_role("admin") || user.locale == "en"`,
			roles:      []string{"user"},
			userMap:    map[string]any{"locale": "en"},
			want:       true,
		},
		{
			name:       "complex OR both false",
			expression: `has_role("admin") || user.locale == "en"`,
			roles:      []string{"user"},
			userMap:    map[string]any{"locale": "ru"},
			want:       false,
		},
		{
			name:       "invalid expression syntax",
			expression: "invalid +++",
			wantErr:    true,
			errContain: "compile expression",
		},
		{
			name:       "non-bool result",
			expression: "1 + 2",
			wantErr:    true,
			errContain: "bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			userMap := tt.userMap
			if userMap == nil {
				userMap = map[string]any{}
			}
			roles := tt.roles
			if roles == nil {
				roles = []string{}
			}

			env := makeEnv(roles, userMap)
			got, err := evaluate(tt.expression, env)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("evaluate() error = nil, want error containing %q", tt.errContain)
				}
				if tt.errContain != "" && !containsStr(err.Error(), tt.errContain) {
					t.Errorf("evaluate() error = %q, want it to contain %q", err.Error(), tt.errContain)
				}
				return
			}

			if err != nil {
				t.Fatalf("evaluate() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// containsStr checks whether s contains substr.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
