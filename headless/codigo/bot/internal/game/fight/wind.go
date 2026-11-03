package fight

import (
	"maps"

	"ddbot/bot/internal/game/value"
)

const (
	WindSourceDisplay = "display"
	WindSourceFire    = "fire"
)

type Wind struct {
	Tenths        *int32
	Current       bool
	Source        *string
	Direction     *int32
	Display       *[3]int8
	Pictures      map[int8]WindPicture
	PictureID     *int8
	PictureSHA256 *string
}

func (w Wind) Clone() Wind {
	w.Tenths = value.Copy(w.Tenths)
	w.Source = value.Copy(w.Source)
	w.Direction = value.Copy(w.Direction)
	w.Display = value.Copy(w.Display)
	w.Pictures = maps.Clone(w.Pictures)
	w.PictureID = value.Copy(w.PictureID)
	w.PictureSHA256 = value.Copy(w.PictureSHA256)

	return w
}

func (w *Wind) Expire() {
	w.Tenths = nil
	w.Current = false
	w.Source = nil
}
