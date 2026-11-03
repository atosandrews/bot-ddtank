# Local CAPTCHA recognition model

Source: [sml2h3/ddddocr](https://github.com/sml2h3/ddddocr), PyPI package ddddocr 1.6.1, `common.onnx` (the beta model). MIT license is retained in LICENSE. The matching 8,210-entry character table was exported from CharsetManager._get_beta_charset as charset.json.

Model SHA-256: `33b5cd351ee94e73a6bf8fa18c415ed8b819b3ffd342e267c30d8ad8334e34e8`.

Input `input1`: float32 `[1,1,64,width]`. Go converts the bounded PNG to grayscale, resizes with bilinear interpolation and normalizes pixels to `[-1,1]`. Output `387` contains sequence logits. Decode uses CTC blank/repetition removal across the full character set. Only four ASCII digits with every character's peak softmax confidence above the configured threshold produce an automatic answer. Scores are model estimates, not guaranteed probabilities of correctness.

This model is independent of DDTank client assets. No captured challenge is embedded in it. Bot loads it locally through ONNX Runtime; Python and ddddocr are not runtime dependencies. No model or challenge is downloaded/uploaded at runtime.
