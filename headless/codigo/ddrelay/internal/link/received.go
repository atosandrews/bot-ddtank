package link

import pb "ddbot/api/gen/go/ddbot/v1"

type received struct {
	message *pb.BotMessage
	err     error
}
