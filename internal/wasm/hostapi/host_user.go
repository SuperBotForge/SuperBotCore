package hostapi

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
	"github.com/vmihailenco/msgpack/v5"
)

type userInfoRequest struct {
	UserID int64 `msgpack:"user_id"`
}

type userInfoResponse struct {
	ID         int64  `msgpack:"id"`
	FullName   string `msgpack:"full_name,omitempty"`
	ExternalID string `msgpack:"external_id,omitempty"`
	IsTeacher  bool   `msgpack:"is_teacher,omitempty"`
}

type usersInfoRequest struct {
	UserIDs []int64 `msgpack:"user_ids"`
}

type userPositionResponse struct {
	PositionType    string `msgpack:"position_type"`
	Status          string `msgpack:"status,omitempty"`
	NationalityType string `msgpack:"nationality_type,omitempty"`
	FundingType     string `msgpack:"funding_type,omitempty"`
	EducationForm   string `msgpack:"education_form,omitempty"`
	GroupCode       string `msgpack:"group_code,omitempty"`
	GroupName       string `msgpack:"group_name,omitempty"`
	ProgramName     string `msgpack:"program_name,omitempty"`
	StreamName      string `msgpack:"stream_name,omitempty"`
}

type userInfoFullResponse struct {
	ID         int64                  `msgpack:"id"`
	FullName   string                 `msgpack:"full_name,omitempty"`
	ExternalID string                 `msgpack:"external_id,omitempty"`
	IsTeacher  bool                   `msgpack:"is_teacher,omitempty"`
	Positions  []userPositionResponse `msgpack:"positions,omitempty"`
}

type usersInfoResponse struct {
	Users []userInfoFullResponse `msgpack:"users"`
}

func (h *HostAPI) userInfoFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		offset := uint32(stack[0])
		length := uint32(stack[1])

		data, err := readPayload(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req userInfoRequest
		if err := msgpack.Unmarshal(data, &req); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if req.UserID <= 0 {
			returnError(ctx, mod, stack, fmt.Errorf("user_id must be greater than zero"))
			return
		}

		pluginID := pluginIDFromContext(ctx)
		if err := h.perms.CheckPermission(pluginID, "user_info"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if h.deps.UserProvider == nil {
			returnError(ctx, mod, stack, errDepNotAvailable("UserProvider"))
			return
		}

		info, err := h.deps.UserProvider.GetUserInfo(ctx, req.UserID)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		writeResult(ctx, mod, stack, userInfoResponse{
			ID:         info.ID,
			FullName:   info.FullName,
			ExternalID: info.ExternalID,
			IsTeacher:  info.IsTeacher,
		})
	}
}

func (h *HostAPI) usersInfoFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		offset := uint32(stack[0])
		length := uint32(stack[1])

		data, err := readPayload(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req usersInfoRequest
		if err := msgpack.Unmarshal(data, &req); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		pluginID := pluginIDFromContext(ctx)
		if err := h.perms.CheckPermission(pluginID, "user_info"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if h.deps.UserProvider == nil {
			returnError(ctx, mod, stack, errDepNotAvailable("UserProvider"))
			return
		}

		users, err := h.deps.UserProvider.GetUsersInfo(ctx, req.UserIDs)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		resp := usersInfoResponse{Users: make([]userInfoFullResponse, len(users))}
		for i, u := range users {
			positions := make([]userPositionResponse, len(u.Positions))
			for j, p := range u.Positions {
				positions[j] = userPositionResponse{
					PositionType:    p.PositionType,
					Status:          p.Status,
					NationalityType: p.NationalityType,
					FundingType:     p.FundingType,
					EducationForm:   p.EducationForm,
					GroupCode:       p.GroupCode,
					GroupName:       p.GroupName,
					ProgramName:     p.ProgramName,
					StreamName:      p.StreamName,
				}
			}
			resp.Users[i] = userInfoFullResponse{
				ID:         u.ID,
				FullName:   u.FullName,
				ExternalID: u.ExternalID,
				IsTeacher:  u.IsTeacher,
				Positions:  positions,
			}
		}
		writeResult(ctx, mod, stack, resp)
	}
}
