package app

import "go.uber.org/fx"

type fakeLifecycle struct{ hooks []fx.Hook }

func (l *fakeLifecycle) Append(hook fx.Hook) { l.hooks = append(l.hooks, hook) }
