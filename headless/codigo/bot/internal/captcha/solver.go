package captcha

import "context"

type Solver interface {
	Solve(context.Context, string) (Result, error)
}
