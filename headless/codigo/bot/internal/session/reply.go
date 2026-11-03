package session

import pb "ddbot/api/gen/go/ddbot/v1"

type reply struct {
	messages []*pb.BotMessage
	err      error
}
