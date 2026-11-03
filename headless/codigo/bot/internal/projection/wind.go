package projection

import (
	"ddbot/bot/internal/game/fight"
	"ddbot/bot/internal/game/value"
)

func resolveWindDisplay(state *fight.State) {
	w := &state.Wind
	w.Expire()
	if w.Display == nil || w.Direction == nil {
		return
	}

	whole := w.Pictures[w.Display[0]].Glyph
	separator := w.Pictures[w.Display[1]].Glyph
	fraction := w.Pictures[w.Display[2]].Glyph
	if separator != "." || len(whole) != 1 || len(fraction) != 1 {
		return
	}

	if whole[0] < '0' || whole[0] > '9' || fraction[0] < '0' || fraction[0] > '9' {
		return
	}

	tenths := (int32(whole[0]-'0')*10 + int32(fraction[0]-'0')) * *w.Direction
	w.Tenths = &tenths
	w.Current = true
	w.Source = value.Of(fight.WindSourceDisplay)
}
