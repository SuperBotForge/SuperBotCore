package providers

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"SuperBotGo/internal/authz"
)

type UniversityProvider struct {
	pool *pgxpool.Pool
}

func NewUniversityProvider(pool *pgxpool.Pool) *UniversityProvider {
	return &UniversityProvider{pool: pool}
}

func (p *UniversityProvider) LoadAttributes(ctx context.Context, sc *authz.SubjectContext) error {
	if sc.ExternalID == "" {
		return nil
	}

	var natType, fundType, eduForm *string
	err := p.pool.QueryRow(ctx, `
		SELECT sp.nationality_type, sp.funding_type, sp.education_form
		FROM student_positions sp
		JOIN persons pe ON pe.id = sp.person_id
		WHERE pe.external_id = $1 AND sp.status = 'active'
		LIMIT 1
	`, sc.ExternalID).Scan(&natType, &fundType, &eduForm)
	if err != nil {
		// No active student position — not an error, just skip attributes.
		return nil
	}

	if natType != nil {
		sc.Attrs["nationality_type"] = *natType
	}
	if fundType != nil {
		sc.Attrs["funding_type"] = *fundType
	}
	if eduForm != nil {
		sc.Attrs["education_form"] = *eduForm
	}

	return nil
}

func (p *UniversityProvider) ContributeConditions(ctx context.Context) []authz.RuleConditionType {
	return []authz.RuleConditionType{
		p.buildAttributeType(),
		p.buildGraphType(ctx),
	}
}

func (p *UniversityProvider) ContributeFieldValues(ctx context.Context) map[string][]authz.RuleParamOption {
	fv := map[string][]authz.RuleParamOption{
		"nationality_type": {{Value: "domestic", Label: "domestic"}, {Value: "foreign", Label: "foreign"}},
		"funding_type":     {{Value: "budget", Label: "budget"}, {Value: "contract", Label: "contract"}},
		"education_form":   {{Value: "full_time", Label: "full_time"}, {Value: "part_time", Label: "part_time"}, {Value: "remote", Label: "remote"}},
		"primary_channel":  {{Value: "TELEGRAM", Label: "TELEGRAM"}, {Value: "DISCORD", Label: "DISCORD"}, {Value: "VK", Label: "VK"}, {Value: "MATTERMOST", Label: "MATTERMOST"}, {Value: "WEB", Label: "WEB"}},
	}

	if vals := p.loadDistinctValues(ctx, "student_positions", "nationality_type"); len(vals) > 0 {
		fv["nationality_type"] = vals
	}
	if vals := p.loadDistinctValues(ctx, "student_positions", "funding_type"); len(vals) > 0 {
		fv["funding_type"] = vals
	}
	if vals := p.loadDistinctValues(ctx, "student_positions", "education_form"); len(vals) > 0 {
		fv["education_form"] = vals
	}
	if vals := p.loadDistinctValues(ctx, "channel_accounts", "channel_type"); len(vals) > 0 {
		fv["primary_channel"] = vals
	}

	return fv
}

func (p *UniversityProvider) buildAttributeType() authz.RuleConditionType {
	fields := []authz.RuleParamOption{
		{Value: "nationality_type", Label: "Гражданство"},
		{Value: "funding_type", Label: "Финансирование"},
		{Value: "education_form", Label: "Форма обучения"},
		{Value: "primary_channel", Label: "Канал"},
		{Value: "locale", Label: "Локаль"},
		{Value: "external_id", Label: "Внешний ID"},
	}

	return authz.RuleConditionType{
		ID:       "attribute",
		Label:    "Атрибут пользователя",
		Template: `user.{field} {operator} "{value}"`,
		Params: []authz.RuleParam{
			{
				Name:    "field",
				Label:   "Поле",
				Type:    "select",
				Options: fields,
			},
			{
				Name:  "operator",
				Label: "Оператор",
				Type:  "select",
				Options: []authz.RuleParamOption{
					{Value: "==", Label: "="},
					{Value: "!=", Label: "!="},
				},
			},
			{
				Name:        "value",
				Label:       "Значение",
				Type:        "text_or_select",
				Placeholder: "значение...",
				DependsOn:   "field",
			},
		},
	}
}

func (p *UniversityProvider) buildGraphType(ctx context.Context) authz.RuleConditionType {
	relations := []authz.RuleParamOption{
		{Value: "member", Label: "member (член)"},
		{Value: "teacher", Label: "teacher (преподаватель)"},
		{Value: "foreign_teacher", Label: "foreign_teacher (преп. иностранцев)"},
		{Value: "dean", Label: "dean (декан)"},
		{Value: "head", Label: "head (завкаф)"},
		{Value: "director", Label: "director (руководитель)"},
		{Value: "curator", Label: "curator (куратор)"},
	}

	if vals := p.loadDistinctRelations(ctx); len(vals) > 0 {
		relations = vals
	}

	objectTypes := []authz.RuleParamOption{
		{Value: "faculty", Label: "Факультет"},
		{Value: "department", Label: "Кафедра"},
		{Value: "program", Label: "Направление"},
		{Value: "stream", Label: "Поток"},
		{Value: "study_group", Label: "Группа"},
		{Value: "subgroup", Label: "Подгруппа"},
	}

	if vals := p.loadDistinctObjectTypes(ctx); len(vals) > 0 {
		objectTypes = vals
	}

	return authz.RuleConditionType{
		ID:       "graph",
		Label:    "Проверка по графу",
		Template: `check("{relation}", "{objectType}", "{objectId}")`,
		Params: []authz.RuleParam{
			{
				Name:    "relation",
				Label:   "Связь",
				Type:    "select",
				Options: relations,
			},
			{
				Name:    "objectType",
				Label:   "Тип объекта",
				Type:    "select",
				Options: objectTypes,
			},
			{
				Name:        "objectId",
				Label:       "Код объекта",
				Type:        "text",
				Placeholder: "код...",
				DependsOn:   "objectType",
			},
		},
	}
}

func (p *UniversityProvider) loadDistinctValues(ctx context.Context, table, column string) []authz.RuleParamOption {
	allowedColumns := map[string]bool{
		"student_positions:nationality_type": true,
		"student_positions:funding_type":     true,
		"student_positions:education_form":   true,
		"channel_accounts:channel_type":      true,
	}
	key := table + ":" + column
	if !allowedColumns[key] {
		return nil
	}

	rows, err := p.pool.Query(ctx,
		"SELECT DISTINCT "+column+" FROM "+table+" WHERE "+column+" IS NOT NULL ORDER BY "+column)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var opts []authz.RuleParamOption
	for rows.Next() {
		var val string
		if rows.Scan(&val) == nil {
			opts = append(opts, authz.RuleParamOption{Value: val, Label: val})
		}
	}
	return opts
}

// Relations and object types are derived from the SpiceDB schema (deployments/schema.zed)
// rather than querying the authorization_tuples table.

func (p *UniversityProvider) loadDistinctRelations(_ context.Context) []authz.RuleParamOption {
	return []authz.RuleParamOption{
		{Value: "member", Label: "member (член)"},
		{Value: "teacher", Label: "teacher (преподаватель)"},
		{Value: "foreign_teacher", Label: "foreign_teacher (преп. иностранцев)"},
		{Value: "dean", Label: "dean (декан)"},
		{Value: "head", Label: "head (завкаф)"},
		{Value: "director", Label: "director (руководитель)"},
		{Value: "curator", Label: "curator (куратор)"},
		{Value: "staff", Label: "staff (сотрудник)"},
		{Value: "owner", Label: "owner (владелец)"},
	}
}

func (p *UniversityProvider) loadDistinctObjectTypes(_ context.Context) []authz.RuleParamOption {
	return []authz.RuleParamOption{
		{Value: "faculty", Label: "Факультет"},
		{Value: "department", Label: "Кафедра"},
		{Value: "program", Label: "Направление"},
		{Value: "stream", Label: "Поток"},
		{Value: "study_group", Label: "Группа"},
		{Value: "subgroup", Label: "Подгруппа"},
		{Value: "nationality_category", Label: "Категория гражданства"},
	}
}

var (
	_ authz.AttributeProvider = (*UniversityProvider)(nil)
	_ authz.SchemaContributor = (*UniversityProvider)(nil)
)
