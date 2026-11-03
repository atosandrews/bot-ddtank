package session

import pb "ddbot/api/gen/go/ddbot/v1"

type observationReply struct {
	snapshot *pb.GetSessionObservationResponse
	err      error
}
