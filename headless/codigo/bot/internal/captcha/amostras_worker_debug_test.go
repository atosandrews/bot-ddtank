package captcha

// Diagnóstico local: o worker Python do bot-ddtank nos captchas reais salvos.

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAmostrasWorker(t *testing.T) {
	dir, script := os.Getenv("AMOSTRAS"), os.Getenv("OCR_WORKER")
	if dir == "" || script == "" {
		t.Skip("AMOSTRAS/OCR_WORKER não definidos")
	}
	w, err := NewWorker("python3", "-I", script)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	files, _ := filepath.Glob(filepath.Join(dir, "*.png"))
	for _, f := range files {
		data, _ := os.ReadFile(f)
		esperado := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(f), "codigo_real_"), ".png")
		for tentativa := 1; tentativa <= 2; tentativa++ {
			res, err := w.Solve(context.Background(), base64.StdEncoding.EncodeToString(data))
			t.Logf("%s tentativa=%d esperado=%s lido=%q confianca=%.3f erro=%v", filepath.Base(f), tentativa, esperado, res.Text, res.Confidence, err)
		}
	}
}
