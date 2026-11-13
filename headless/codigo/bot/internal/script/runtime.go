package script

import (
	"context"
	"io"
	"time"

	lua "github.com/yuin/gopher-lua"

	"ddbot/bot/internal/captcha"
	"ddbot/bot/internal/game"
)

type Runtime struct {
	captchaSolver captcha.Solver
	captchaJob    *captchaJob
	loading       bool
	state         *lua.LState
	session       *game.Session
	hooks         *hooks
	output        io.Writer
	timeout       time.Duration
	started       time.Time
	report        *StatusReport
	// luaCache: tabelas Lua já convertidas de fight.state()/fight.actors(), refeitas só quando a
	// view do jogo muda (10/11/2026: montar e converter a tabela a cada tick era ~25% da CPU e,
	// com o GC, deixava o Formigueiro no Fly acima da CPU garantida, que então era segurada).
	luaCache map[string]cachedLua
}

type cachedLua struct {
	version uint64
	value   lua.LValue
}

// pushCached reaproveita a tabela Lua enquanto a view não muda. Cada chamada recebe uma cópia
// rasa do nível de cima (mudar um campo dela não afeta a próxima leitura); as tabelas internas
// (atores, cartas, turno) são compartilhadas e os scripts só as leem.
func (r *Runtime) pushCached(key string, build func() any) int {
	version := r.session.ViewVersion()
	if cached, ok := r.luaCache[key]; ok && cached.version == version {
		r.state.Push(shallowCopy(r.state, cached.value))

		return 1
	}

	converted, err := r.convert(build())
	if err != nil {
		return fail(r.state, err)
	}

	if r.luaCache == nil {
		r.luaCache = map[string]cachedLua{}
	}

	r.luaCache[key] = cachedLua{version: version, value: converted}
	r.state.Push(shallowCopy(r.state, converted))

	return 1
}

func shallowCopy(l *lua.LState, value lua.LValue) lua.LValue {
	table, ok := value.(*lua.LTable)
	if !ok {
		return value
	}

	copia := l.CreateTable(table.Len(), 0)
	table.ForEach(func(k, v lua.LValue) { copia.RawSet(k, v) })

	return copia
}

func NewRuntime(session *game.Session, source []byte, output io.Writer, timeout time.Duration) (*Runtime, error) {
	return NewRuntimeWithOptions(session, source, output, timeout, Options{})
}

func NewRuntimeWithOptions(session *game.Session, source []byte, output io.Writer, timeout time.Duration, options Options) (*Runtime, error) {
	if session == nil || output == nil || timeout <= 0 {
		return nil, &Error{Operation: "configure", Reason: "session, output and a positive timeout are required"}
	}

	state := lua.NewState(lua.Options{SkipOpenLibs: true})
	r := &Runtime{
		captchaSolver: options.CaptchaSolver,
		loading:       true,
		state:         state,
		session:       session,
		output:        output,
		timeout:       timeout,
		started:       time.Now(),
	}

	for _, open := range []lua.LGFunction{lua.OpenBase, lua.OpenTable, lua.OpenString, lua.OpenMath} {
		open(state)
	}

	for _, name := range []string{"dofile", "loadfile", "load", "loadstring", "collectgarbage", "print", "module", "require"} {
		state.SetGlobal(name, lua.LNil)
	}

	r.bind()
	state.SetTop(0)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	state.SetContext(ctx)
	if err := r.bindLibrary(options.Library); err != nil {
		r.Close()

		return nil, err
	}

	if err := r.loadModules(options.Modules); err != nil {
		r.Close()

		return nil, err
	}

	if err := state.DoString(string(source)); err != nil {
		r.Close()

		return nil, &Error{Operation: "load", Cause: err}
	}

	hooks, err := readHooks(state)
	if err != nil {
		r.Close()

		return nil, err
	}

	r.loading = false
	r.hooks = hooks
	state.SetTop(0)
	state.RemoveContext()

	return r, nil
}

func (r *Runtime) Close() {
	r.cancelCaptcha()
	r.state.Close()
}

func (r *Runtime) Event(ctx context.Context, name string, fields map[string]any) error {
	return r.call(ctx, "on_event", r.hooks.event, map[string]any{"name": name, "fields": fields})
}

func (r *Runtime) Packet(ctx context.Context, packet map[string]any) error {
	return r.call(ctx, "on_packet", r.hooks.packet, packet)
}

func (r *Runtime) Stop(ctx context.Context, reason string) error {
	return r.call(ctx, "on_close", r.hooks.close, map[string]any{"reason": reason})
}

func (r *Runtime) call(ctx context.Context, name string, hook *lua.LFunction, data map[string]any) error {
	if hook == nil {
		return nil
	}

	value, err := r.convert(data)
	if err != nil {
		return err
	}

	deadline, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	r.state.SetContext(deadline)
	defer r.state.RemoveContext()
	if err := r.state.CallByParam(lua.P{Fn: hook, NRet: 0, Protect: true}, value); err != nil {
		return &Error{Operation: name, Cause: err}
	}

	return nil
}
