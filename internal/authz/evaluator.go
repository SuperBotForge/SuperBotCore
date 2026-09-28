package authz

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	authzed "github.com/authzed/authzed-go/v1"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

type exprEnv struct {
	User map[string]any `expr:"user"`

	Check      func(relation, objectType, objectID string) (bool, error) `expr:"check"`
	IsMember   func(objectType, objectID string) (bool, error)           `expr:"is_member"`
	HasRole    func(roleName string) bool                                `expr:"has_role"`
	HasAnyRole func(roleNames ...string) bool                            `expr:"has_any_role"`
}

func buildUserMap(sc *SubjectContext) map[string]any {
	userMap := map[string]any{
		"id":              int64(sc.UserID),
		"external_id":     sc.ExternalID,
		"groups":          sc.Groups,
		"roles":           sc.Roles,
		"primary_channel": sc.PrimaryChannel,
		"locale":          sc.Locale,
	}
	for k, v := range sc.Attrs {
		userMap[k] = v
	}
	return userMap
}

func buildExprEnv(ctx context.Context, sc *SubjectContext, client *authzed.Client) exprEnv {
	roleSet := make(map[string]bool, len(sc.Roles))
	for _, r := range sc.Roles {
		roleSet[r] = true
	}

	checkSpice := func(permission, objectType, objectID string) (bool, error) {
		// University subjects use the linked person's external ID. Never fall
		// back to a bot ID: it may equal another person's external ID.
		subjectID := strconv.FormatInt(int64(sc.UserID), 10)
		if objectType == "group" {
			objectType = "study_group"
		} // legacy rule alias
		switch objectType {
		case "faculty", "department", "program", "stream", "study_group", "subgroup", "nationality_category", "student_record":
			subjectID = sc.ExternalID
			if subjectID == "" {
				return false, nil
			}
		}
		if client == nil {
			return false, fmt.Errorf("graph authorization is not configured")
		}
		resp, err := client.CheckPermission(ctx, &v1.CheckPermissionRequest{
			Resource:    &v1.ObjectReference{ObjectType: objectType, ObjectId: objectID},
			Permission:  permission,
			Subject:     &v1.SubjectReference{Object: &v1.ObjectReference{ObjectType: "user", ObjectId: subjectID}},
			Consistency: &v1.Consistency{Requirement: &v1.Consistency_FullyConsistent{FullyConsistent: true}},
		})
		if err != nil {
			return false, fmt.Errorf("graph check %s on %s:%s: %w", permission, objectType, objectID, err)
		}
		return resp.Permissionship == v1.CheckPermissionResponse_PERMISSIONSHIP_HAS_PERMISSION, nil
	}

	return exprEnv{
		User: buildUserMap(sc),

		Check: checkSpice,

		IsMember: func(objectType, objectID string) (bool, error) {
			return checkSpice("member", objectType, objectID)
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

// EvalWithContext evaluates a policy expression using SpiceDB for graph checks.
func EvalWithContext(ctx context.Context, expression string, sc *SubjectContext, client *authzed.Client) (bool, error) {
	env := buildExprEnv(ctx, sc, client)
	return evaluate(expression, env)
}

const maxCompiledExprs = 1024

var (
	compiledExprs sync.Map // expression string -> *vm.Program
	compiledCount atomic.Int64
)

func evaluate(expression string, env exprEnv) (bool, error) {
	var program *vm.Program

	if cached, ok := compiledExprs.Load(expression); ok {
		program = cached.(*vm.Program)
	} else {
		compiled, err := expr.Compile(expression, expr.Env(env), expr.AsBool())
		if err != nil {
			return false, fmt.Errorf("compile expression: %w", err)
		}
		if compiledCount.Load() >= maxCompiledExprs {
			// Evict all entries to prevent unbounded growth.
			compiledExprs.Clear()
			compiledCount.Store(0)
		}
		compiledExprs.Store(expression, compiled)
		compiledCount.Add(1)
		program = compiled
	}

	result, err := expr.Run(program, env)
	if err != nil {
		return false, fmt.Errorf("run expression: %w", err)
	}

	b, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("expression must return bool, got %T", result)
	}
	return b, nil
}
