package contract

import pb "ddbot/api/gen/go/ddbot/v1"

func ValidateBatch(v *pb.CommandBatch) error {
	if v == nil || v.Id == 0 || v.Generation == 0 || len(v.Commands) == 0 || len(v.Commands) > MaxCommands {
		return &Error{Field: "batch", Reason: "identity, generation and 1 to 64 commands required"}
	}

	var previous uint64

	for _, command := range v.Commands {
		if err := ValidateCommand(command); err != nil {
			return err
		}

		if command.Id <= previous {
			return &Error{Field: "batch", Reason: "command IDs must increase"}
		}

		previous = command.Id
	}

	return nil
}
