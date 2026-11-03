package projection

import "ddbot/bot/internal/game/fight"

func ReadFightResults(data map[string]any) ([]fight.ResultPlayer, error) {
	f := &fields{data: data}
	rows := f.list("players")
	if f.err != nil {
		return nil, f.err
	}

	results := make([]fight.ResultPlayer, 0, len(rows))
	for _, row := range rows {
		child := &fields{data: row}
		result := fight.ResultPlayer{
			PlayerID: child.integer("player_id"),
			GainedGP: child.integer("gained_gp"),
			Winner:   child.flag("winner"),
		}
		if child.err != nil {
			return nil, child.err
		}

		results = append(results, result)
	}

	return results, nil
}
