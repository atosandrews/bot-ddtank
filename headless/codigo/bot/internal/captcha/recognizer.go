package captcha

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

type Recognizer struct {
	mutex   sync.RWMutex
	engine  *ort.DynamicAdvancedSession
	classes []string
	minimum float64
	gate    chan struct{}
	closed  bool
}

func NewRecognizer(library, model, charset string, minimum float64) (*Recognizer, error) {
	if library == "" || model == "" || charset == "" || minimum <= 0 || minimum > 1 || math.IsNaN(minimum) {
		return nil, &Error{Operation: "configure", Reason: "library, model, charset and confidence in (0, 1] are required"}
	}

	data, err := os.ReadFile(charset)
	if err != nil {
		return nil, &Error{Operation: "configure", Reason: "cannot read model charset", Cause: err}
	}

	var classes []string
	if err := json.Unmarshal(data, &classes); err != nil {
		return nil, &Error{Operation: "configure", Reason: "invalid model charset", Cause: err}
	}

	if len(classes) != 8210 || classes[0] != "" {
		return nil, &Error{Operation: "configure", Reason: "unexpected model charset"}
	}

	ort.SetSharedLibraryPath(library)
	if err := ort.InitializeEnvironment(); err != nil {
		return nil, &Error{Operation: "initialize", Reason: "cannot load ONNX Runtime", Cause: err}
	}

	options, err := ort.NewSessionOptions()
	if err != nil {
		return nil, &Error{Operation: "initialize", Reason: "cannot create model options", Cause: errors.Join(err, ort.DestroyEnvironment())}
	}

	if err := options.SetIntraOpNumThreads(1); err != nil {
		return nil, &Error{Operation: "initialize", Reason: "cannot limit model threads", Cause: errors.Join(err, options.Destroy(), ort.DestroyEnvironment())}
	}

	engine, loadErr := ort.NewDynamicAdvancedSession(model, []string{"input1"}, []string{"387"}, options)
	if err := errors.Join(loadErr, options.Destroy()); err != nil {
		if engine != nil {
			err = errors.Join(err, engine.Destroy())
		}

		return nil, &Error{Operation: "initialize", Reason: "cannot load CAPTCHA model", Cause: errors.Join(err, ort.DestroyEnvironment())}
	}

	return &Recognizer{engine: engine, classes: classes, minimum: minimum, gate: make(chan struct{}, 1)}, nil
}

func (r *Recognizer) Close() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.closed {
		return nil
	}

	r.closed = true
	if err := errors.Join(r.engine.Destroy(), ort.DestroyEnvironment()); err != nil {
		return &Error{Operation: "close", Reason: "cannot release model runtime", Cause: err}
	}

	return nil
}

func (r *Recognizer) Solve(ctx context.Context, encoded string) (result Result, failure error) {
	defer func() {
		if failure != nil {
			failure = &Error{Operation: "solve", Reason: failure.Error(), Cause: failure}
		}
	}()

	if err := ctx.Err(); err != nil {
		return result, &Error{Operation: "recognize", Reason: "recognition cancelled", Cause: err}
	}
	select {
	case r.gate <- struct{}{}:
		defer func() { <-r.gate }()
	case <-ctx.Done():
		return result, &Error{Operation: "recognize", Reason: "recognition cancelled", Cause: ctx.Err()}
	}

	r.mutex.RLock()
	defer r.mutex.RUnlock()
	if r.closed {
		return result, &Error{Operation: "recognize", Reason: "model runtime is closed"}
	}

	data, width, err := imageTensor(encoded)
	if err != nil {
		return result, err
	}

	input, err := ort.NewTensor(ort.NewShape(1, 1, imageHeight, int64(width)), data)
	if err != nil {
		return result, &Error{Operation: "recognize", Reason: "cannot create input tensor", Cause: err}
	}
	defer func() { failure = errors.Join(failure, input.Destroy()) }()

	options, err := ort.NewRunOptions()
	if err != nil {
		return result, &Error{Operation: "recognize", Reason: "cannot create inference options", Cause: err}
	}
	defer func() { failure = errors.Join(failure, options.Destroy()) }()

	cancelled := make(chan error, 1)
	stop := context.AfterFunc(ctx, func() { cancelled <- options.Terminate() })
	defer func() {
		if !stop() {
			failure = errors.Join(failure, <-cancelled)
		}
	}()

	output := []ort.Value{nil}
	defer func() {
		if output[0] != nil {
			failure = errors.Join(failure, output[0].Destroy())
		}
	}()
	if err := r.engine.RunWithOptions([]ort.Value{input}, output, options); err != nil {
		return result, &Error{Operation: "recognize", Reason: "model inference failed", Cause: err}
	}

	if err := ctx.Err(); err != nil {
		return result, &Error{Operation: "recognize", Reason: "recognition cancelled", Cause: err}
	}

	tensor, ok := output[0].(*ort.Tensor[float32])
	if !ok {
		return result, &Error{Operation: "decode", Reason: "expected floating point model output"}
	}

	return decode(tensor.GetData(), r.classes, r.minimum)
}
