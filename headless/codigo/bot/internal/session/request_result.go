package session

import pb "ddbot/api/gen/go/ddbot/v1"

type requestResult struct {
	hash   [32]byte
	result *pb.ScriptResult
	size   int
}
