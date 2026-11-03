package script

import (
	"slices"

	"ddbot/bot/internal/game/fight"
)

func windFields(w fight.Wind) map[string]any {
	var display []int8
	if w.Display != nil {
		display = append(display, w.Display[:]...)
	}

	var number any
	if w.Current && w.Tenths != nil {
		number = float64(*w.Tenths) / 10
	}

	ids := make([]int8, 0, len(w.Pictures))

	for id := range w.Pictures {
		ids = append(ids, id)
	}

	slices.Sort(ids)
	pictures := make([]map[string]any, 0, len(ids))

	for _, id := range ids {
		picture := w.Pictures[id]
		row := map[string]any{"id": id, "sha256": picture.SHA256}
		if picture.Glyph != "" {
			row["glyph"] = picture.Glyph
		}

		pictures = append(pictures, row)
	}

	return map[string]any{
		"value": number, "tenths": optional(w.Tenths), "current": w.Current,
		"source": optional(w.Source), "direction": optional(w.Direction), "display": display,
		"pictures": pictures, "picture_id": optional(w.PictureID), "picture_sha256": optional(w.PictureSHA256),
	}
}
