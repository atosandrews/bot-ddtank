package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func startBot(t *testing.T, library string, dial func(context.Context, string) (net.Conn, error)) (*grpc.ClientConn, context.Context) {
	t.Helper()
	temp := t.TempDir()
	binary := filepath.Join(temp, "ddbot")
	build := exec.Command("go", "build", "-o", binary, "./bot/cmd/ddbot")
	build.Dir = "../../.."
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build bot: %v %s", err, output)
	}

	config := filepath.Join(temp, "bot.yaml")
	if err := os.WriteFile(config, []byte("listen: 127.0.0.1:0\nlua_dir: "+library+"\ntenants:\n  - id: test\n    api_key_env: DDBOT_TEST_KEY\n"), 0600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	process := exec.CommandContext(ctx, binary, "serve", "-config", config)
	process.Env = append(os.Environ(), "DDBOT_TEST_KEY=synthetic-test-api-key")
	stderr, err := process.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := process.Start(); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		cancel()
		if err := process.Wait(); err != nil && ctx.Err() == nil {
			t.Error(err)
		}
	})
	scanner := bufio.NewScanner(stderr)
	if !scanner.Scan() {
		t.Fatal("bot did not start")
	}

	var startup struct {
		Message string `json:"msg"`
		Address string `json:"address"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &startup); err != nil {
		t.Fatal(err)
	}

	if startup.Message != "bot listening" || startup.Address == "" {
		t.Fatalf("bot startup: %s", scanner.Text())
	}

	address := startup.Address
	cc, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDisableRetry(), grpc.WithContextDialer(dial))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := cc.Close(); err != nil {
			t.Error(err)
		}
	})

	return cc, ctx
}
