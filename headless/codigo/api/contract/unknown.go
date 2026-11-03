package contract

import "google.golang.org/protobuf/reflect/protoreflect"

func rejectUnknown(message protoreflect.Message, depth int) error {
	if depth > 3*MaxDepth+16 || len(message.GetUnknown()) != 0 {
		return &Error{Field: "message", Reason: "unknown fields or nesting limit exceeded"}
	}

	var failure error
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.IsMap() {
			if field.MapValue().Kind() == protoreflect.MessageKind {
				value.Map().Range(func(_ protoreflect.MapKey, item protoreflect.Value) bool {
					failure = rejectUnknown(item.Message(), depth+1)

					return failure == nil
				})
			}

			return failure == nil
		}

		if field.IsList() {
			if field.Kind() == protoreflect.MessageKind {
				list := value.List()

				for i := 0; i < list.Len(); i++ {
					if err := rejectUnknown(list.Get(i).Message(), depth+1); err != nil {
						failure = err
						break
					}
				}
			}

			return failure == nil
		}

		if field.Kind() == protoreflect.MessageKind {
			failure = rejectUnknown(value.Message(), depth+1)
		}

		return failure == nil
	})

	return failure
}
