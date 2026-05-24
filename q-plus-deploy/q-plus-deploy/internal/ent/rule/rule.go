package rule

import (
	"context"
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/marktable"
	"q+/internal/generated/ent/privacy"
)

const DiscordRuleCtxKey = "discord"
const CronRuleCtxKey = "cron"
const AllowedCourseIdCtxKey = "allowedCourseId"

func AllowIfDiscordCommand() privacy.QueryMutationRule {
	return privacy.ContextQueryMutationRule(func(ctx context.Context) error {
		if ctx.Value(DiscordRuleCtxKey) != nil {
			return privacy.Allow
		}
		return nil
	})
}

func AllowIfCronTask() privacy.QueryMutationRule {
	return privacy.ContextQueryMutationRule(func(ctx context.Context) error {
		if ctx.Value(CronRuleCtxKey) != nil {
			return privacy.Allow
		}
		return nil
	})
}

func QueueTemplateAllowOnlyCertainCourseId() privacy.MutationRule {
	return privacy.QueueTemplateMutationRuleFunc(func(ctx context.Context, m *ent.QueueTemplateMutation) error {
		courseId, exists := m.CourseInstanceID()
		if !exists {
			return privacy.Denyf("missing course instance ID in mutation")
		}
		if courseId != ctx.Value(AllowedCourseIdCtxKey) {
			return privacy.Denyf("course instance id %d is not allowed", courseId)
		}
		return privacy.Allow
	})

}

func MarkTableTabAllowOnlyCertainCourseId() privacy.MutationRule {
	return privacy.MarkTableTabMutationRuleFunc(func(ctx context.Context, m *ent.MarkTableTabMutation) error {
		markTableId, exists := m.MarkTableID()
		if !exists {
			return privacy.Denyf("missing mark table ID in mutation")
		}
		courseId, err := m.Client().MarkTable.Query().Where(marktable.ID(markTableId)).QueryCourseInstance().OnlyID(ctx)
		if err != nil {
			return privacy.Denyf("mark table %d not found", markTableId)
		}
		if courseId != ctx.Value(AllowedCourseIdCtxKey) {
			return privacy.Denyf("course instance id %d is not allowed", courseId)
		}
		return privacy.Allow
	})
}
