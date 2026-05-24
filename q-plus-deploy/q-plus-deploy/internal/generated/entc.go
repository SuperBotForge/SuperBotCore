package main

import (
	"ariga.io/ogent"
	"entgo.io/contrib/entoas"
	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
	"github.com/ogen-go/ogen"
	"log"
)

var noPagination = gen.MustParse(gen.NewTemplate("").Parse(`
{{ define "ogent/ogent/helper/list/paginate" }}
// Skip pagination
{{ end }}
`))

func main() {
	spec := new(ogen.Spec)
	oas, err := entoas.NewExtension(
		entoas.Spec(spec),
		entoas.DefaultPolicy(entoas.PolicyExclude),
		entoas.SimpleModels(),
		entoas.Mutations(
			func(graph *gen.Graph, spec *ogen.Spec) error {
				spec.AddPathItem("/schedule-queues", ogen.NewPathItem().
					SetDescription("Schedule queues").
					SetPost(ogen.NewOperation().
						SetOperationID("scheduleQueues").
						SetSummary("Schedule queues").
						SetRequestBody(
							ogen.NewRequestBody().SetRequired(true).SetJSONContent(
								ogen.NewSchema().AddRequiredProperties(
									ogen.Int64().ToProperty("queue_template_id"),
									ogen.DateTime().ToProperty("start_time"),
									ogen.DateTime().ToProperty("end_time"),
								).AsArray(),
							),
						).
						AddResponse("200", ogen.NewResponse().SetDescription("Queues created").
							SetJSONContent(spec.RefSchema("Queue").Schema.AsArray()),
						).
						AddResponse("400", ogen.NewResponse().SetDescription("Bad request")),
					),
				)
				return nil
			}),
	)
	if err != nil {
		log.Fatalf("creating entoas extension: %v", err)
	}
	ogentExt, err := ogent.NewExtension(spec, ogent.Templates(noPagination))
	if err != nil {
		log.Fatalf("creating ogent extension: %v", err)
	}
	err = entc.Generate("../ent/schema", &gen.Config{
		Target:  "./ent",
		Package: "q+/internal/generated/ent",
		IDType:  &field.TypeInfo{Type: field.TypeInt64},
	},
		entc.Extensions(ogentExt, oas),
		entc.FeatureNames("sql/upsert", "privacy", "schema/snapshot"),
	)
	if err != nil {
		log.Fatalf("running ent codegen: %v", err)
	}
}
