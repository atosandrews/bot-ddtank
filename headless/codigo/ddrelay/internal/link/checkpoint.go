package link

import pb "ddbot/api/gen/go/ddbot/v1"

type Checkpoint struct {
	HighWater uint64
	Receipts  []*pb.CommandReceipt
}
