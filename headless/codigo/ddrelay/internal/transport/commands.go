package transport

import (
	"context"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/relay/internal/framing"
	"ddbot/relay/internal/protocol"
)

func (c *Connection) execute(ctx context.Context, batch *pb.CommandBatch) error {
	if batch == nil || batch.Id == 0 || len(batch.Commands) == 0 || len(batch.Commands) > contract.MaxCommands {
		c.automation.Invalidate("invalid command batch")

		return nil
	}

	state := c.automation.State()
	reason := pb.FailureCode_FAILURE_CODE_UNSPECIFIED
	if !state.Attached || state.ChangingScript || batch.Generation != state.Generation || batch.ScriptGeneration != state.ScriptGeneration {
		reason = pb.FailureCode_FAILURE_CODE_STALE_BINDING
	} else if batch.BaseSequence != c.sequence {
		reason = pb.FailureCode_FAILURE_CODE_STALE_STATE
	} else if c.closing || !c.relay.Ready() {
		reason = pb.FailureCode_FAILURE_CODE_PACKET_BOUNDARY_UNAVAILABLE
	}

	previous := c.highWater
	frames := make([]framing.Frame, len(batch.Commands))

	for i, command := range batch.Commands {
		if command == nil || command.Id == 0 {
			c.automation.Invalidate("command ID required")

			return nil
		}

		if command.Id <= previous {
			reason = pb.FailureCode_FAILURE_CODE_COMMAND_ALREADY_SEEN
		}

		previous = command.Id
		frame, err := protocol.Encode(command)
		if err != nil {
			reason = pb.FailureCode_FAILURE_CODE_INVALID_COMMAND
		}

		frames[i] = frame
	}

	if reason != pb.FailureCode_FAILURE_CODE_UNSPECIFIED {
		if reason == pb.FailureCode_FAILURE_CODE_STALE_BINDING || reason == pb.FailureCode_FAILURE_CODE_STALE_STATE {
			c.staleRejections++
		}

		for _, command := range batch.Commands {
			c.reject(batch.Id, command.Id, reason)
		}

		return nil
	}

	for i, frame := range frames {
		command := batch.Commands[i]
		if current := c.automation.State(); !current.Attached || current.ChangingScript || current.Generation != batch.Generation || current.ScriptGeneration != batch.ScriptGeneration {
			for _, remaining := range batch.Commands[i:] {
				c.remember(&pb.CommandReceipt{
					BatchId:   batch.Id,
					CommandId: remaining.Id,
					Status:    pb.ReceiptStatus_RECEIPT_STATUS_CANCELLED,
					Error:     &pb.Failure{Code: pb.FailureCode_FAILURE_CODE_SCRIPT_OR_BINDING_CHANGED, Message: "command cancelled before write"},
				})
			}

			return nil
		}

		c.highWater = command.Id
		n, record, err := c.send(ctx, frame)
		receipt := &pb.CommandReceipt{
			BatchId:      batch.Id,
			CommandId:    command.Id,
			Status:       pb.ReceiptStatus_RECEIPT_STATUS_SENT,
			BytesWritten: uint32(n),
		}
		if err != nil {
			receipt.Status = pb.ReceiptStatus_RECEIPT_STATUS_UNCERTAIN
			receipt.Error = &pb.Failure{Code: pb.FailureCode_FAILURE_CODE_WRITE_FAILED, Message: "game write did not complete"}
		}

		c.remember(receipt)
		if err == nil {
			if observeErr := c.observeCommand(ctx, record, command.Id); observeErr != nil {
				return observeErr
			}
		}

		if err != nil {
			for _, remaining := range batch.Commands[i+1:] {
				c.remember(&pb.CommandReceipt{
					BatchId:   batch.Id,
					CommandId: remaining.Id,
					Status:    pb.ReceiptStatus_RECEIPT_STATUS_CANCELLED,
					Error:     &pb.Failure{Code: pb.FailureCode_FAILURE_CODE_WRITE_FAILED, Message: "previous command failed"},
				})
			}

			return err
		}
	}

	return nil
}

func (c *Connection) reject(batch, id uint64, reason pb.FailureCode) {
	for _, receipt := range c.receipts {
		if receipt.CommandId == id {
			if receipt.BatchId == batch {
				c.automation.Receipt(receipt)

				return
			}

			c.automation.Invalidate("command ID reused in another batch")

			return
		}
	}

	message, err := contract.FailureCodeName(reason)
	if err != nil {
		c.automation.Invalidate("invalid command rejection")

		return
	}

	status := pb.ReceiptStatus_RECEIPT_STATUS_REJECTED
	if id <= c.highWater {
		status = pb.ReceiptStatus_RECEIPT_STATUS_UNCERTAIN
		reason = pb.FailureCode_FAILURE_CODE_OUTCOME_EXPIRED
		message = "old command outcome no longer retained"
	}

	c.remember(&pb.CommandReceipt{
		BatchId:   batch,
		CommandId: id,
		Status:    status,
		Error:     &pb.Failure{Code: reason, Message: message},
	})
}

func (c *Connection) remember(receipt *pb.CommandReceipt) {
	if receipt.Status == pb.ReceiptStatus_RECEIPT_STATUS_UNCERTAIN {
		c.uncertainCommands++
	}

	if receipt.CommandId > c.highWater {
		c.highWater = receipt.CommandId
	}

	c.receipts = append(c.receipts, receipt)
	if len(c.receipts) > 128 {
		c.receipts = c.receipts[len(c.receipts)-128:]
	}

	c.automation.Receipt(receipt)
}
