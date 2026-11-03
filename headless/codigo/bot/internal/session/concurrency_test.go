package session

import (
	"context"
	"crypto/sha256"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/script"
)

func TestConcurrentCallbacksEvaluationsAndCloseStayWithOwner(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	options := script.Options{Library: []script.Module{{Name: "shared", Source: []byte(`return {nested={value=0},jobs={}}`)}}}
	owners := make([]*Session, 2)

	for i := range owners {
		owner, err := New(fmt.Sprintf("owner-%d", i), 1, options, time.Second)
		if err != nil {
			t.Fatal(err)
		}

		defer owner.Close()
		owners[i] = owner
		if _, err := owner.Bind(ctx, 1, nil); err != nil {
			t.Fatal(err)
		}

		source := fmt.Sprintf(`counter=0;packets=0;return {api_version=1,on_packet=function() packets=packets+1 end,on_close=function() console.log("owner-%d") end}`, i)
		hash := sha256.Sum256([]byte(source))
		messages, err := owner.Process(ctx, 1, &pb.RelayMessage{Payload: &pb.RelayMessage_Load{Load: &pb.LoadScript{RequestId: "hooks", Source: source, Sha256: hash[:]}}})
		if err != nil || messages[0].GetResult().Error != nil {
			t.Fatalf("load=%v error=%v", messages, err)
		}
	}

	var workers sync.WaitGroup
	failures := make(chan error, 102)

	for _, owner := range owners {
		workers.Add(1)
		go func() {
			defer workers.Done()

			for sequence := uint64(1); sequence <= 20; sequence++ {
				_, err := owner.Process(ctx, 1, &pb.RelayMessage{Payload: &pb.RelayMessage_Packet{Packet: &pb.PacketEvent{
					Sequence:    sequence,
					Direction:   pb.PacketDirection_PACKET_DIRECTION_SERVER_TO_CLIENT,
					Origin:      pb.PacketOrigin_PACKET_ORIGIN_NETWORK,
					Compression: pb.Compression_COMPRESSION_NONE,
					Header:      &pb.Header{Code: 167},
					Status:      pb.DecodeStatus_DECODE_STATUS_UNKNOWN,
					Fields:      &pb.Object{},
				}}})
				if err != nil {
					failures <- err

					return
				}
			}
		}()

		for i := 0; i < 50; i++ {
			workers.Add(1)
			go func() {
				defer workers.Done()
				messages, err := owner.Process(ctx, 1, &pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: fmt.Sprintf("eval-%d", i), Source: `counter=counter+1;return counter`}}})
				if err != nil {
					failures <- err

					return
				}

				if messages[0].GetResult().Error != nil {
					failures <- fmt.Errorf("evaluation: %v", messages[0])

					return
				}
			}()
		}
	}

	workers.Wait()
	close(failures)

	for err := range failures {
		t.Error(err)
	}

	evaluate := func(owner *Session, id, source string) any {
		t.Helper()
		messages, err := owner.Process(ctx, 1, &pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: id, Source: source}}})
		if err != nil || messages[0].GetResult().Error != nil {
			t.Fatalf("evaluation=%v error=%v", messages, err)
		}

		value, err := contract.DecodeValue(messages[0].GetResult().Value)
		if err != nil {
			t.Fatal(err)
		}

		return value
	}

	for _, owner := range owners {
		if value := evaluate(owner, "counts", `return {counter=counter,packets=packets}`); !reflect.DeepEqual(value, map[string]any{"counter": float64(50), "packets": float64(20)}) {
			t.Fatalf("counts=%v", value)
		}
	}

	evaluate(owners[0], "mutate", `local data=require("shared");data.nested.value=99;data.jobs[1]={tested=17};return true`)
	if value := evaluate(owners[1], "independent", `local data=require("shared");return data.nested.value==0 and data.jobs[1]==nil`); value != true {
		t.Fatalf("library crossed sessions=%v", value)
	}

	messages, err := owners[0].Process(ctx, 1, &pb.RelayMessage{Payload: &pb.RelayMessage_Close{Close: &pb.Close{Reason: "test"}}})
	if err != nil || len(messages) != 1 || messages[0].GetLog().Message != "owner-0\n" {
		t.Fatalf("close=%v error=%v", messages, err)
	}

	if value := evaluate(owners[1], "after-close", `return session.info().valid and counter==50 and packets==20`); value != true {
		t.Fatalf("close crossed sessions=%v", value)
	}
}
