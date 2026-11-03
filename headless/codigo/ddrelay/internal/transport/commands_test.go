package transport

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/relay/internal/framing"
	"ddbot/relay/internal/link"
)

func TestPartialCommandWriteIsUncertainAndNeverRepeated(t *testing.T) {
	for name, command := range map[string]*pb.Command{
		"room": {Id: 1, Payload: &pb.Command_RoomStart{RoomStart: &pb.RoomStart{}}},
		"pow":  {Id: 1, Payload: &pb.Command_Pow{Pow: &pb.Pow{}}},
		"prop": {Id: 1, Payload: &pb.Command_UseProp{UseProp: &pb.UseProp{BagType: 1, Place: -1, TemplateId: 10004}}},
	} {
		t.Run(name, func(t *testing.T) {
			automation := &fakeAutomation{state: link.State{Attached: true, Generation: 1}}
			socket := &partialSocket{}
			c := &Connection{
				client:     socket,
				server:     socket,
				recorder:   &memoryRecorder{},
				automation: automation,
				relay:      framing.NewRelay(),
			}
			key := [8]byte{3, 19, 47, 61, 89, 103, 131, 157}
			first := encrypted(t, &key, framing.Frame{Header: framing.Header{Code: 4}})
			second := encrypted(t, &key, framing.Frame{Header: framing.Header{Code: 6}})
			if _, err := c.relay.Forward(append(first, second...)); err != nil {
				t.Fatal(err)
			}

			batch := &pb.CommandBatch{Id: 1, Generation: 1, Commands: []*pb.Command{command, {Id: 2, Payload: &pb.Command_RoomLeave{RoomLeave: &pb.RoomLeave{}}}}}
			if err := c.execute(context.Background(), batch); !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("write error %v", err)
			}

			if len(socket.written) != 3 || len(automation.receipts) != 2 || automation.receipts[0].Status != pb.ReceiptStatus_RECEIPT_STATUS_UNCERTAIN || automation.receipts[0].BytesWritten != 3 || automation.receipts[1].Status != pb.ReceiptStatus_RECEIPT_STATUS_CANCELLED || len(automation.events) != 0 {
				t.Fatalf("partial write bytes=%d receipts=%v events=%v", len(socket.written), automation.receipts, automation.events)
			}

			if err := c.execute(context.Background(), batch); err != nil {
				t.Fatal(err)
			}

			if len(socket.written) != 3 || !reflect.DeepEqual(automation.receipts[:2], automation.receipts[2:]) {
				t.Fatalf("duplicate write %d %v", len(socket.written), automation.receipts)
			}
		})
	}
}

func TestStaleBatchNeverWrites(t *testing.T) {
	for _, test := range []struct {
		name                         string
		generation, sequence, script uint64
		reason                       pb.FailureCode
	}{{"old binding", 2, 4, 0, pb.FailureCode_FAILURE_CODE_STALE_BINDING}, {"old event", 1, 3, 0, pb.FailureCode_FAILURE_CODE_STALE_STATE}, {"old script", 1, 4, 1, pb.FailureCode_FAILURE_CODE_STALE_BINDING}} {
		t.Run(test.name, func(t *testing.T) {
			automation := &fakeAutomation{state: link.State{Attached: true, Generation: 1}}
			socket := &partialSocket{}
			c := &Connection{server: socket, automation: automation, relay: framing.NewRelay(), sequence: 4}
			batch := &pb.CommandBatch{
				Id:               1,
				Generation:       test.generation,
				BaseSequence:     test.sequence,
				ScriptGeneration: test.script,
				Commands: []*pb.Command{
					{Id: 1, Payload: &pb.Command_Pow{Pow: &pb.Pow{}}},
					{Id: 2, Payload: &pb.Command_UseProp{UseProp: &pb.UseProp{BagType: 1, Place: -1, TemplateId: 10004}}},
				},
			}
			if err := c.execute(context.Background(), batch); err != nil {
				t.Fatal(err)
			}

			if len(socket.written) != 0 || len(automation.receipts) != 2 || automation.receipts[0].Status != pb.ReceiptStatus_RECEIPT_STATUS_REJECTED || automation.receipts[0].Error.Code != test.reason || automation.receipts[1].Status != pb.ReceiptStatus_RECEIPT_STATUS_REJECTED || automation.receipts[1].Error.Code != test.reason || len(automation.events) != 0 {
				t.Fatalf("stale batch bytes=%d receipts=%v", len(socket.written), automation.receipts)
			}
		})
	}
}
