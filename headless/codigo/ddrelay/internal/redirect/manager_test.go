package redirect

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeRunner struct {
	inputs   []string
	present  bool
	applyErr error
	table    string
}

func (r *fakeRunner) Run(ctx context.Context, input string, args ...string) ([]byte, error) {
	if len(args) > 0 && args[0] == "-j" {
		if r.present {
			table := r.table
			if table == "" {
				table = "ddtank_1000_11401_00000000000000000000000000000000"
			}

			return []byte(`{"nftables":[{"table":{"family":"ip","name":"` + table + `"}}]}`), nil
		}

		return []byte(`{"nftables":[]}`), nil
	}

	r.inputs = append(r.inputs, input)
	if strings.HasPrefix(input, "add table") {
		r.present = true
		r.table = strings.Fields(input)[3]

		return nil, r.applyErr
	}

	r.present = false

	return nil, nil
}

func TestRedirectOwnsOnlyItsTable(t *testing.T) {
	runner := &fakeRunner{}
	manager, err := NewManager(Config{
		UID:   1000,
		Port:  11401,
		Ports: []uint16{1401},
		Mark:  0xdd7a,
	}, "00000000000000000000000000000000", runner)
	if err != nil {
		t.Fatal(err)
	}

	if err := manager.Install(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := manager.Close(context.Background()); err != nil {
		t.Fatal(err)
	}

	table := "ddtank_1000_11401_00000000000000000000000000000000"
	want := "add table ip " + table + "\nadd chain ip " + table + " output { type nat hook output priority dstnat; policy accept; }\nadd rule ip " + table + " output meta skuid 1000 meta mark != 56698 tcp dport { 1401 } counter redirect to :11401\n"
	if len(runner.inputs) != 2 || runner.inputs[0] != want || runner.inputs[1] != "delete table ip "+table+"\n" {
		t.Fatalf("transactions = %#v", runner.inputs)
	}
}

func TestRedirectCleansAmbiguousInstallAndRefusesCollision(t *testing.T) {
	for _, tt := range []struct {
		name         string
		present      bool
		applyErr     error
		transactions int
	}{
		{"collision", true, nil, 0},
		{"ambiguous apply", false, errors.New("interrupted"), 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{present: tt.present, applyErr: tt.applyErr}
			manager, err := NewManager(Config{
				UID:   1000,
				Port:  11401,
				Ports: []uint16{1401},
				Mark:  1,
			}, "00000000000000000000000000000000", runner)
			if err != nil {
				t.Fatal(err)
			}

			if err := manager.Install(context.Background()); err == nil {
				t.Fatal("accepted failed installation")
			}

			if len(runner.inputs) != tt.transactions {
				t.Fatalf("transactions = %#v", runner.inputs)
			}
		})
	}
}
