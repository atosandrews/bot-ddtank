package grpcserver

import pb "ddbot/api/gen/go/ddbot/v1"

type received struct {
	message *pb.RelayMessage
	err     error
}
