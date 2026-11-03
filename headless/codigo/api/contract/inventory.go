package contract

func ValidateInventoryOrganization(bag int32, places []int32) error {
	start := int32(0)
	switch bag {
	case 0:
		start = 31
	case 1:
	default:
		return &Error{Field: "bag_type", Reason: "only permanent bags 0 and 1 can be organized"}
	}

	if len(places) == 0 || len(places) > 256 {
		return &Error{Field: "places", Reason: "expected 1 to 256 occupied slots"}
	}

	seen := make(map[int32]bool, len(places))
	for _, place := range places {
		if place < start || place > 1023 || seen[place] {
			return &Error{Field: "places", Reason: "duplicate, equipped or out-of-range slot"}
		}

		seen[place] = true
	}

	return nil
}
