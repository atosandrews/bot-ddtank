package grpcserver

import (
	"context"

	"google.golang.org/grpc"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func receive(ctx context.Context, stream grpc.BidiStreamingServer[pb.RelayMessage, pb.BotMessage], incoming chan<- received) {
	for {
		message, err := stream.Recv()

		select {
		case incoming <- received{message: message, err: err}:
		case <-ctx.Done():
			return
		}

		if err != nil {
			return
		}
	}

}

func transmit(ctx context.Context, stream grpc.BidiStreamingServer[pb.RelayMessage, pb.BotMessage], outgoing <-chan *pb.BotMessage, sendError chan<- error) {
	for {
		select {
		case message := <-outgoing:
			if err := stream.Send(message); err != nil {
				sendError <- err

				return
			}
		case <-ctx.Done():
			return
		}
	}

}
