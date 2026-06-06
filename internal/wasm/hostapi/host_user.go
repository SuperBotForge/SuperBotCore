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
	ID       int64  `msgpack:"id"`
	FullName string `msgpack:"full_name,omitempty"`
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
			ID:       info.ID,
			FullName: info.FullName,
		})
	}
}
