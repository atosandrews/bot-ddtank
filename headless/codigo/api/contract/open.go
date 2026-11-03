package contract

import pb "ddbot/api/gen/go/ddbot/v1"

func ValidateOpen(v *pb.OpenSession) error {
	if v == nil {
		return &Error{Field: "open", Reason: "opening message required"}
	}

	if v.ApiVersion != APIVersion || v.CodecRevision != CodecRevision {
		return &Error{Field: "version", Reason: "unsupported API or codec revision"}
	}

	for _, s := range []string{v.RelayInstanceId, v.GameSessionId} {
		if len(s) == 0 || len(s) > 128 {
			return &Error{Field: "identity", Reason: "1 to 128 bytes required"}
		}
	}

	if len(v.RelayId) > 128 || v.LocalStream == 0 {
		return &Error{Field: "identity", Reason: "invalid label or local stream"}
	}

	return nil
}
