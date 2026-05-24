package web

import (
	"context"
	"github.com/samber/do/v2"
	"github.com/samber/lo"
	"net/http"
	"q+/internal/core"
	"q+/internal/generated/ent"
	"q+/internal/generated/ent/ogent"
)

type MyOgentHandler struct {
	*ogent.OgentHandler
	core *core.Core
}

func NewMyOgentHandler(i do.Injector) *MyOgentHandler {
	c := do.MustInvoke[*core.Core](i)
	client := do.MustInvoke[*ent.Client](i)
	return &MyOgentHandler{
		OgentHandler: ogent.NewOgentHandler(client),
		core:         c,
	}
}

func (h *MyOgentHandler) CreateMarkTableTab(ctx context.Context, req *ogent.CreateMarkTableTabReq) (ogent.CreateMarkTableTabRes, error) {
	markTableTab, err := core.CreateMarkTableTab(useCaseContext(ctx, h, core.CreateMarkTableTabParams{
		MarkTableId: req.MarkTable,
		Name:        req.Name,
	}))
	if err != nil {
		return nil, err
	}
	return ogent.NewMarkTableTab(markTableTab), nil
}

func (h *MyOgentHandler) ScheduleQueues(ctx context.Context, req []ogent.ScheduleQueuesReqItem) (ogent.ScheduleQueuesRes, error) {
	queues, err := core.ScheduleQueues(useCaseContext(ctx, h, core.ScheduleQueuesParams{
		Queues: lo.Map(req, func(item ogent.ScheduleQueuesReqItem, _ int) core.ScheduleQueueItemParams {
			return core.ScheduleQueueItemParams{
				QueueTemplateId: item.QueueTemplateID,
				StartTime:       item.StartTime,
				EndTime:         item.EndTime,
			}
		}),
	}))
	if err != nil {
		return nil, err
	}

	var res ogent.ScheduleQueuesOKApplicationJSON = ogent.NewQueues(queues)
	return &res, nil
}

// ListQueueTemplateQueues handles GET /queue-templates/{id}/queues requests.
func (h *MyOgentHandler) ListQueueTemplateQueues(ctx context.Context, params ogent.ListQueueTemplateQueuesParams) (ogent.ListQueueTemplateQueuesRes, error) {
	es, err := core.ListQueueTemplateQueues(useCaseContext(ctx, h, core.ListQueueTemplateQueuesParams{
		QueueTemplateId: params.ID,
	}))
	if err != nil {
		switch {
		case ent.IsNotFound(err):
			return &ogent.R404{
				Code:   http.StatusNotFound,
				Status: http.StatusText(http.StatusNotFound),
				Errors: rawError(err),
			}, nil
		case ent.IsNotSingular(err):
			return &ogent.R409{
				Code:   http.StatusConflict,
				Status: http.StatusText(http.StatusConflict),
				Errors: rawError(err),
			}, nil
		default:
			// Let the server handle the error.
			return nil, err
		}
	}
	r := ogent.NewQueues(es)
	return (*ogent.ListQueueTemplateQueuesOKApplicationJSON)(&r), nil
}
