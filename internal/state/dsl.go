package state

import "SuperBotGo/internal/model"

type CommandNode interface {
	commandNode()
}

type OptionsPage struct {
	Options []model.Option
	HasMore bool
	Error   string
}

type PaginationConfig struct {
	Prompt         string
	Prompts        map[string]string
	PromptProvider func(ctx StepContext) string
	PageSize       int
	PageProvider   func(ctx StepContext, page int) OptionsPage
}

type StepNode struct {
	ParamName            string
	MessageBuilder       func(ctx StepContext) model.Message
	Validate             func(model.UserInput) bool
	ValidateWithContext  func(ctx StepContext, input model.UserInput) bool
	Condition            func(model.OptionMap) bool
	ConditionWithContext func(ctx StepContext) bool
	Pagination           *PaginationConfig
}

func (StepNode) commandNode() {}

type BranchNode struct {
	OnParam string
	Cases   map[string][]CommandNode
	Default []CommandNode
}

func (BranchNode) commandNode() {}

type ConditionalCase struct {
	Predicate            func(model.OptionMap) bool
	PredicateWithContext func(ctx StepContext) bool
	Nodes                []CommandNode
}

type ConditionalBranchNode struct {
	Cases   []ConditionalCase
	Default []CommandNode
}

func (ConditionalBranchNode) commandNode() {}

type CommandDefinition struct {
	Name            string
	Descriptions    map[string]string
	Description     string // Deprecated: use Descriptions for user-facing command text.
	Requirements    *model.RoleRequirements
	Nodes           []CommandNode
	PreservesDialog bool
}

func (cd *CommandDefinition) ResolveActiveSteps(ctx StepContext) []StepNode {
	return flattenNodes(cd.Nodes, ctx)
}

func (cd *CommandDefinition) CurrentStep(ctx StepContext) *StepNode {
	steps := cd.ResolveActiveSteps(ctx)
	for i := range steps {
		if _, exists := ctx.Params[steps[i].ParamName]; !exists {
			return &steps[i]
		}
	}
	return nil
}

func (cd *CommandDefinition) IsComplete(ctx StepContext) bool {
	return cd.CurrentStep(ctx) == nil
}

func flattenNodes(nodes []CommandNode, ctx StepContext) []StepNode {
	var result []StepNode
	for _, node := range nodes {
		switch n := node.(type) {
		case StepNode:
			switch {
			case n.ConditionWithContext != nil:
				if n.ConditionWithContext(ctx) {
					result = append(result, n)
				}
			case n.Condition == nil || n.Condition(ctx.Params):
				result = append(result, n)
			}
		case BranchNode:
			value, exists := ctx.Params[n.OnParam]
			var branch []CommandNode
			if exists {
				if caseNodes, ok := n.Cases[value]; ok {
					branch = caseNodes
				} else {
					branch = n.Default
				}
			} else {
				branch = n.Default
			}
			if branch != nil {
				result = append(result, flattenNodes(branch, ctx)...)
			}
		case ConditionalBranchNode:
			var matched []CommandNode
			for _, c := range n.Cases {
				matchedCase := false
				switch {
				case c.PredicateWithContext != nil:
					if c.PredicateWithContext(ctx) {
						matched = c.Nodes
						matchedCase = true
					}
				case c.Predicate != nil && c.Predicate(ctx.Params):
					matched = c.Nodes
					matchedCase = true
				}
				if matchedCase {
					break
				}
			}
			if matched == nil {
				matched = n.Default
			}
			if matched != nil {
				result = append(result, flattenNodes(matched, ctx)...)
			}
		}
	}
	return result
}
