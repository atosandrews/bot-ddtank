package contract

import (
	"fmt"
	"math"
	"reflect"
	"unicode/utf8"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func EncodeValue(value any) (*pb.Value, error) {
	budget := MaxMessageBytes

	return encodeValue(value, 0, &budget)
}

func encodeValue(value any, depth int, budget *int) (*pb.Value, error) {
	*budget -= 16
	if depth > MaxDepth || *budget < 0 {
		return nil, &Error{Field: "value", Reason: "size or nesting limit exceeded"}
	}

	result := &pb.Value{}
	if value == nil {
		result.Kind = &pb.Value_NullValue{NullValue: true}

		return result, nil
	}

	v := reflect.ValueOf(value)

	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		result.Kind = &pb.Value_SignedValue{SignedValue: v.Int()}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		result.Kind = &pb.Value_UnsignedValue{UnsignedValue: v.Uint()}
	case reflect.Float32, reflect.Float64:
		n := v.Float()
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return nil, &Error{Field: "number", Reason: "finite number required"}
		}
		result.Kind = &pb.Value_NumberValue{NumberValue: n}
	case reflect.Bool:
		result.Kind = &pb.Value_BoolValue{BoolValue: v.Bool()}
	case reflect.String:
		s := v.String()
		*budget -= len(s)
		if !utf8.ValidString(s) {
			return nil, &Error{Field: "string", Reason: "UTF-8 required"}
		}
		result.Kind = &pb.Value_StringValue{StringValue: s}
	case reflect.Slice, reflect.Array:
		if bytes, ok := value.([]byte); ok {
			*budget -= len(bytes)
			if *budget < 0 {
				return nil, &Error{Field: "bytes", Reason: "size limit exceeded"}
			}

			result.Kind = &pb.Value_BytesValue{BytesValue: append([]byte{}, bytes...)}
			break
		}
		if v.Len() > *budget/16 {
			return nil, &Error{Field: "list", Reason: "size limit exceeded"}
		}
		list := &pb.List{Values: make([]*pb.Value, 0, v.Len())}
		for i := 0; i < v.Len(); i++ {
			child, err := encodeValue(v.Index(i).Interface(), depth+1, budget)
			if err != nil {
				return nil, err
			}

			list.Values = append(list.Values, child)
		}
		result.Kind = &pb.Value_ListValue{ListValue: list}
	case reflect.Map:
		object, ok := value.(map[string]any)
		if !ok {
			return nil, &Error{Field: "object", Reason: "string keys required"}
		}
		if len(object) > *budget/16 {
			return nil, &Error{Field: "object", Reason: "size limit exceeded"}
		}
		fields := make(map[string]*pb.Value, len(object))
		for key, item := range object {
			*budget -= len(key)
			if !utf8.ValidString(key) {
				return nil, &Error{Field: "key", Reason: "UTF-8 required"}
			}

			child, err := encodeValue(item, depth+1, budget)
			if err != nil {
				return nil, err
			}

			fields[key] = child
		}
		result.Kind = &pb.Value_ObjectValue{ObjectValue: &pb.Object{Fields: fields}}
	default:
		return nil, &Error{Field: "value", Reason: fmt.Sprintf("unsupported type %T", value)}
	}

	if *budget < 0 {
		return nil, &Error{Field: "value", Reason: "size limit exceeded"}
	}

	return result, nil
}

func DecodeValue(value *pb.Value) (any, error) {
	if value != nil {
		if err := rejectUnknown(value.ProtoReflect(), 0); err != nil {
			return nil, err
		}
	}

	budget := MaxMessageBytes

	return decodeValue(value, 0, &budget)
}

func decodeValue(value *pb.Value, depth int, budget *int) (any, error) {
	*budget -= 16
	if value == nil || depth > MaxDepth || *budget < 0 {
		return nil, &Error{Field: "value", Reason: "missing value or size/nesting limit exceeded"}
	}

	if len(value.ProtoReflect().GetUnknown()) != 0 {
		return nil, &Error{Field: "value", Reason: "unknown fields"}
	}

	switch v := value.Kind.(type) {
	case *pb.Value_NullValue:
		if !v.NullValue {
			return nil, &Error{Field: "null", Reason: "null marker must be true"}
		}
		return nil, nil
	case *pb.Value_SignedValue:
		return v.SignedValue, nil
	case *pb.Value_UnsignedValue:
		return v.UnsignedValue, nil
	case *pb.Value_NumberValue:
		if math.IsNaN(v.NumberValue) || math.IsInf(v.NumberValue, 0) {
			return nil, &Error{Field: "number", Reason: "finite number required"}
		}
		return v.NumberValue, nil
	case *pb.Value_BoolValue:
		return v.BoolValue, nil
	case *pb.Value_StringValue:
		*budget -= len(v.StringValue)
		if *budget < 0 || !utf8.ValidString(v.StringValue) {
			return nil, &Error{Field: "string", Reason: "invalid UTF-8 or size limit exceeded"}
		}
		return v.StringValue, nil
	case *pb.Value_BytesValue:
		*budget -= len(v.BytesValue)
		if *budget < 0 {
			return nil, &Error{Field: "bytes", Reason: "size limit exceeded"}
		}
		return append([]byte{}, v.BytesValue...), nil
	case *pb.Value_ObjectValue:
		if v.ObjectValue == nil || len(v.ObjectValue.Fields) > *budget/16 {
			return nil, &Error{Field: "object", Reason: "missing object or size limit exceeded"}
		}
		result := make(map[string]any, len(v.ObjectValue.Fields))
		for key, item := range v.ObjectValue.Fields {
			*budget -= len(key)
			if !utf8.ValidString(key) {
				return nil, &Error{Field: "key", Reason: "invalid UTF-8"}
			}

			child, err := decodeValue(item, depth+1, budget)
			if err != nil {
				return nil, err
			}

			result[key] = child
		}
		return result, nil
	case *pb.Value_ListValue:
		if v.ListValue == nil || len(v.ListValue.Values) > *budget/16 {
			return nil, &Error{Field: "list", Reason: "missing list or size limit exceeded"}
		}
		result := make([]any, 0, len(v.ListValue.Values))
		for _, item := range v.ListValue.Values {
			child, err := decodeValue(item, depth+1, budget)
			if err != nil {
				return nil, err
			}

			result = append(result, child)
		}
		return result, nil
	default:
		return nil, &Error{Field: "value", Reason: "missing or unsupported kind"}
	}
}

func EncodeFields(fields map[string]any) (*pb.Object, error) {
	v, err := EncodeValue(fields)
	if err != nil {
		return nil, err
	}

	return v.GetObjectValue(), nil
}

func DecodeFields(fields *pb.Object) (map[string]any, error) {
	v, err := DecodeValue(&pb.Value{Kind: &pb.Value_ObjectValue{ObjectValue: fields}})
	if err != nil {
		return nil, err
	}

	return v.(map[string]any), nil
}
