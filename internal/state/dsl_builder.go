package state

import (
	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/model"
)

type CommandBuilder struct {
	name            string
	description     string
	descriptions    map[string]string
	requirements    *model.RoleRequirements
	nodes           []CommandNode
	preservesDialog bool
}

func NewCommand(name string) *CommandBuilder {
	return &CommandBuilder{name: name}
}

// Description sets a single-locale fallback for older command definitions.
//
// Deprecated: use LocalizedDescription for user-facing command text.
func (b *CommandBuilder) Description(d string) *CommandBuilder {
	b.description = d
	return b
}

// LocalizedDescription sets locale-specific user-facing command text.
func (b *CommandBuilder) LocalizedDescription(descriptions map[string]string) *CommandBuilder {
	b.descriptions = descriptions
	return b
}

func (b *CommandBuilder) PreservesDialog() *CommandBuilder {
	b.preservesDialog = true
	return b
}

func (b *CommandBuilder) RequireRole(systemRole string, globalRoles []string) *CommandBuilder {
	b.requirements = &model.RoleRequirements{
		SystemRole:  systemRole,
		GlobalRoles: globalRoles,
	}
	return b
}

func (b *CommandBuilder) Step(paramName string, configure func(*StepBuilder)) *CommandBuilder {
	sb := &StepBuilder{paramName: paramName}
	configure(sb)
	b.nodes = append(b.nodes, sb.build())
	return b
}

func (b *CommandBuilder) Branch(onParam string, configure func(*BranchBuilder)) *CommandBuilder {
	bb := &BranchBuilder{onParam: onParam}
	configure(bb)
	b.nodes = append(b.nodes, bb.build())
	return b
}

func (b *CommandBuilder) ConditionalBranch(configure func(*ConditionalBranchBuilder)) *CommandBuilder {
	cb := &ConditionalBranchBuilder{}
	configure(cb)
	b.nodes = append(b.nodes, cb.build())
	return b
}

func (b *CommandBuilder) Build() *CommandDefinition {
	return &CommandDefinition{
		Name:            b.name,
		Descriptions:    b.descriptions,
		Description:     b.description,
		Requirements:    b.requirements,
		Nodes:           b.nodes,
		PreservesDialog: b.preservesDialog,
	}
}

type StepBuilder struct {
	paramName      string
	validateFn     func(model.UserInput) bool
	condition      func(model.OptionMap) bool
	blockFactories []func(StepContext) model.ContentBlock
	paginationCfg  *PaginationConfig
}

func (s *StepBuilder) Validate(fn func(model.UserInput) bool) {
	s.validateFn = fn
}

func (s *StepBuilder) VisibleWhen(fn func(model.OptionMap) bool) {
	s.condition = fn
}

func (s *StepBuilder) Prompt(configure func(*PromptBuilder)) {
	pb := &PromptBuilder{}
	configure(pb)
	s.blockFactories = pb.blockFactories
	s.paginationCfg = pb.paginationCfg
}

func (s *StepBuilder) build() StepNode {
	factories := s.blockFactories
	var msgBuilder func(StepContext) model.Message
	if len(factories) > 0 {
		msgBuilder = func(ctx StepContext) model.Message {
			blocks := make([]model.ContentBlock, 0, len(factories))
			for _, f := range factories {
				block := f(ctx)
				if tb, ok := block.(model.TextBlock); ok && tb.Text == "" {
					continue
				}
				blocks = append(blocks, block)
			}
			return model.Message{Blocks: blocks}
		}
	} else {
		msgBuilder = func(_ StepContext) model.Message {
			return model.Message{}
		}
	}

	return StepNode{
		ParamName:      s.paramName,
		MessageBuilder: msgBuilder,
		Validate:       s.validateFn,
		Condition:      s.condition,
		Pagination:     s.paginationCfg,
	}
}

type PromptBuilder struct {
	blockFactories []func(StepContext) model.ContentBlock
	paginationCfg  *PaginationConfig
}

func (p *PromptBuilder) Text(text string, style model.TextStyle) {
	p.blockFactories = append(p.blockFactories, func(_ StepContext) model.ContentBlock {
		return model.TextBlock{Text: text, Style: style}
	})
}

func (p *PromptBuilder) LocalizedText(key string, style model.TextStyle) {
	p.blockFactories = append(p.blockFactories, func(ctx StepContext) model.ContentBlock {
		return model.TextBlock{Text: i18n.Get(key, ctx.Locale), Style: style}
	})
}

func (p *PromptBuilder) TextFromContext(provider func(StepContext) string, style model.TextStyle) {
	p.blockFactories = append(p.blockFactories, func(ctx StepContext) model.ContentBlock {
		return model.TextBlock{Text: provider(ctx), Style: style}
	})
}

func (p *PromptBuilder) Options(prompt string, configure func(*OptionsBuilder)) {
	ob := &OptionsBuilder{}
	configure(ob)
	p.blockFactories = append(p.blockFactories, func(ctx StepContext) model.ContentBlock {
		return model.OptionsBlock{Prompt: prompt, Options: ob.resolve(ctx)}
	})
}

func (p *PromptBuilder) LocalizedOptions(promptKey string, configure func(*OptionsBuilder)) {
	ob := &OptionsBuilder{}
	configure(ob)
	p.blockFactories = append(p.blockFactories, func(ctx StepContext) model.ContentBlock {
		return model.OptionsBlock{Prompt: i18n.Get(promptKey, ctx.Locale), Options: ob.resolve(ctx)}
	})
}

func (p *PromptBuilder) PaginatedOptions(prompt string, pageSize int, provider func(StepContext) []model.Option) {
	p.paginationCfg = &PaginationConfig{
		Prompt:   prompt,
		PageSize: pageSize,
		PageProvider: func(ctx StepContext, page int) OptionsPage {
			all := provider(ctx)
			start := page * pageSize
			if start >= len(all) {
				return OptionsPage{Options: nil, HasMore: false}
			}
			end := start + pageSize
			if end > len(all) {
				end = len(all)
			}
			return OptionsPage{
				Options: all[start:end],
				HasMore: end < len(all),
			}
		},
	}
}

func (p *PromptBuilder) LocalizedPaginatedOptions(promptKey string, pageSize int, provider func(StepContext) []model.Option) {
	p.paginationCfg = &PaginationConfig{
		PromptProvider: func(ctx StepContext) string {
			return i18n.Get(promptKey, ctx.Locale)
		},
		PageSize: pageSize,
		PageProvider: func(ctx StepContext, page int) OptionsPage {
			all := provider(ctx)
			start := page * pageSize
			if start >= len(all) {
				return OptionsPage{Options: nil, HasMore: false}
			}
			end := start + pageSize
			if end > len(all) {
				end = len(all)
			}
			return OptionsPage{
				Options: all[start:end],
				HasMore: end < len(all),
			}
		},
	}
}

func (p *PromptBuilder) PaginatedOptionsWithProvider(prompt string, pageSize int, provider func(StepContext, int) OptionsPage) {
	p.paginationCfg = &PaginationConfig{
		Prompt:       prompt,
		PageSize:     pageSize,
		PageProvider: provider,
	}
}

func (p *PromptBuilder) LocalizedPaginatedOptionsWithProvider(promptKey string, pageSize int, provider func(StepContext, int) OptionsPage) {
	p.paginationCfg = &PaginationConfig{
		PromptProvider: func(ctx StepContext) string {
			return i18n.Get(promptKey, ctx.Locale)
		},
		PageSize:     pageSize,
		PageProvider: provider,
	}
}

func (p *PromptBuilder) Link(url, label string) {
	p.blockFactories = append(p.blockFactories, func(_ StepContext) model.ContentBlock {
		return model.LinkBlock{URL: url, Label: label}
	})
}

func (p *PromptBuilder) Image(url string) {
	p.blockFactories = append(p.blockFactories, func(_ StepContext) model.ContentBlock {
		return model.ImageBlock{URL: url}
	})
}

type OptionsBuilder struct {
	optionBuilders  []func(StepContext) model.Option
	dynamicProvider func(StepContext) []model.Option
}

func (o *OptionsBuilder) Add(label, value string) {
	o.optionBuilders = append(o.optionBuilders, func(_ StepContext) model.Option {
		return model.Option{Label: label, Value: value}
	})
}

func (o *OptionsBuilder) LocalizedOption(key, value string) {
	o.optionBuilders = append(o.optionBuilders, func(ctx StepContext) model.Option {
		return model.Option{Label: i18n.Get(key, ctx.Locale, value), Value: value}
	})
}

func (o *OptionsBuilder) From(provider func() []model.Option) {
	o.dynamicProvider = func(_ StepContext) []model.Option {
		return provider()
	}
}

func (o *OptionsBuilder) FromContext(provider func(StepContext) []model.Option) {
	o.dynamicProvider = provider
}

func (o *OptionsBuilder) resolve(ctx StepContext) []model.Option {
	if o.dynamicProvider != nil {
		return o.dynamicProvider(ctx)
	}
	result := make([]model.Option, len(o.optionBuilders))
	for i, builder := range o.optionBuilders {
		result[i] = builder(ctx)
	}
	return result
}

type BranchBuilder struct {
	onParam      string
	cases        map[string][]CommandNode
	defaultNodes []CommandNode
}

func (b *BranchBuilder) Case(value string, configure func(*NodeListBuilder)) {
	nlb := &NodeListBuilder{}
	configure(nlb)
	if b.cases == nil {
		b.cases = make(map[string][]CommandNode)
	}
	b.cases[value] = nlb.nodes
}

func (b *BranchBuilder) Default(configure func(*NodeListBuilder)) {
	nlb := &NodeListBuilder{}
	configure(nlb)
	b.defaultNodes = nlb.nodes
}

func (b *BranchBuilder) build() BranchNode {
	return BranchNode{
		OnParam: b.onParam,
		Cases:   b.cases,
		Default: b.defaultNodes,
	}
}

type ConditionalBranchBuilder struct {
	cases        []ConditionalCase
	defaultNodes []CommandNode
}

func (c *ConditionalBranchBuilder) Case(predicate func(model.OptionMap) bool, configure func(*NodeListBuilder)) {
	nlb := &NodeListBuilder{}
	configure(nlb)
	c.cases = append(c.cases, ConditionalCase{
		Predicate: predicate,
		Nodes:     nlb.nodes,
	})
}

func (c *ConditionalBranchBuilder) Default(configure func(*NodeListBuilder)) {
	nlb := &NodeListBuilder{}
	configure(nlb)
	c.defaultNodes = nlb.nodes
}

func (c *ConditionalBranchBuilder) build() ConditionalBranchNode {
	return ConditionalBranchNode{
		Cases:   c.cases,
		Default: c.defaultNodes,
	}
}

type NodeListBuilder struct {
	nodes []CommandNode
}

func (n *NodeListBuilder) Step(paramName string, configure func(*StepBuilder)) {
	sb := &StepBuilder{paramName: paramName}
	configure(sb)
	n.nodes = append(n.nodes, sb.build())
}

func (n *NodeListBuilder) Branch(onParam string, configure func(*BranchBuilder)) {
	bb := &BranchBuilder{onParam: onParam}
	configure(bb)
	n.nodes = append(n.nodes, bb.build())
}

func (n *NodeListBuilder) ConditionalBranch(configure func(*ConditionalBranchBuilder)) {
	cb := &ConditionalBranchBuilder{}
	configure(cb)
	n.nodes = append(n.nodes, cb.build())
}
