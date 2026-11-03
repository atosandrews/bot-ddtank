package script

import (
	"context"

	"ddbot/bot/internal/game/action"
)

func (r *Runtime) Action(ctx context.Context, result action.Result) error {
	return r.call(ctx, "on_action", r.hooks.action, map[string]any{
		"id":     result.ID,
		"name":   result.Name,
		"status": result.Status,
		"reason": result.Reason,
	})
}
