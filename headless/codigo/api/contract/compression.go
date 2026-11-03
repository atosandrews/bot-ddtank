package contract

import (
	"fmt"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func CompressionName(value pb.Compression) (string, error) {
	switch value {
	case pb.Compression_COMPRESSION_NONE:
		return "", nil
	case pb.Compression_COMPRESSION_ZLIB:
		return "zlib", nil
	default:
		return "", &Error{Field: "Compression", Reason: fmt.Sprintf("unsupported value %d", value)}
	}
}

func ParseCompression(value string) (pb.Compression, error) {
	switch value {
	case "":
		return pb.Compression_COMPRESSION_NONE, nil
	case "zlib":
		return pb.Compression_COMPRESSION_ZLIB, nil
	default:
		return pb.Compression_COMPRESSION_UNSPECIFIED, &Error{Field: "Compression", Reason: fmt.Sprintf("unsupported value %q", value)}
	}
}
