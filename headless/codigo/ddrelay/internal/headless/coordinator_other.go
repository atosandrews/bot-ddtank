//go:build !linux

package headless

import "context"

func lockCoordinator(context.Context, string) (func(), error) {
	return nil, coordinationError("shared authentication coordination requires Linux", nil)
}
