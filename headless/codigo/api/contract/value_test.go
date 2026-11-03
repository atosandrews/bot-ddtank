package contract

import (
	"math"
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func TestValuesPreserveWireMeaning(t *testing.T) {
	for _, test := range []struct{ input, want any }{
		{int64(math.MinInt64), int64(math.MinInt64)},
		{uint64(math.MaxUint64), uint64(math.MaxUint64)},
		{int32(-5), int64(-5)}, {uint8(255), uint64(255)},
		{nil, nil}, {true, true}, {"á", "á"}, {1.25, 1.25},
		{[]byte{1, 2}, []byte{1, 2}}, {[]any{}, []any{}},
		{map[string]any{"actors": []map[string]any{{"x": int32(-5)}}, "wind": []int8{-1, 0, 3}}, map[string]any{"actors": []any{map[string]any{"x": int64(-5)}}, "wind": []any{int64(-1), int64(0), int64(3)}}},
	} {
		encoded, err := EncodeValue(test.input)
		if err != nil {
			t.Fatal(err)
		}

		wire, err := proto.Marshal(encoded)
		if err != nil {
			t.Fatal(err)
		}

		received := new(pb.Value)
		if err := proto.Unmarshal(wire, received); err != nil {
			t.Fatal(err)
		}

		got, err := DecodeValue(received)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(got, test.want) {
			t.Fatalf("got %#v want %#v", got, test.want)
		}
	}
}

func TestValueLimits(t *testing.T) {
	cyclic := map[string]any{}
	cyclic["self"] = cyclic

	for _, v := range []any{cyclic, math.Inf(1), make([]byte, MaxMessageBytes+1), func() {}} {
		if _, err := EncodeValue(v); err == nil {
			t.Fatalf("accepted %T", v)
		}
	}

	for _, v := range []*pb.Value{nil, {}, {Kind: &pb.Value_ObjectValue{ObjectValue: nil}}, {Kind: &pb.Value_NumberValue{NumberValue: math.NaN()}}} {
		if _, err := DecodeValue(v); err == nil {
			t.Fatalf("accepted %#v", v)
		}
	}
}
