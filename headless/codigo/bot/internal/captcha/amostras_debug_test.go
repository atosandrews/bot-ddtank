package captcha

// Diagnóstico local: lê os captchas reais salvos do projeto Python (nome = resposta esperada).

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAmostrasReais(t *testing.T) {
	dir := os.Getenv("AMOSTRAS")
	if dir == "" {
		t.Skip("AMOSTRAS não definido")
	}
	r, err := NewRecognizer(os.Getenv("DDBOT_ONNXRUNTIME_LIBRARY"), "../../models/captcha/model.onnx", "../../models/captcha/charset.json", 0.75)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	files, _ := filepath.Glob(filepath.Join(dir, "*.png"))
	for _, f := range files {
		data, _ := os.ReadFile(f)
		esperado := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(f), "codigo_real_"), ".png")
		res, err := r.Solve(context.Background(), base64.StdEncoding.EncodeToString(data))
		t.Logf("%s esperado=%s lido=%q confianca=%.3f erro=%v", filepath.Base(f), esperado, res.Text, res.Confidence, err)
	}
}
