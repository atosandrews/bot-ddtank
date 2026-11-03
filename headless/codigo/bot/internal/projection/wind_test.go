package projection

import (
	"reflect"
	"testing"

	"ddbot/bot/internal/game"
	"ddbot/bot/internal/game/fight"
	"ddbot/bot/internal/game/value"
)

const (
	windDotHash   = "6ecd0f9889bd4f5b504e985621c7172f8f5ecddd0e03e7da94e3561170864e93"
	windZeroHash  = "44579aa1ba4ca80f045371694eae00ed404583ff20ca46b805fb91011804ee4d"
	windTwoHash   = "460f0746913c3f3126a0bd4716e6fa8b075685d370b1cd7055965bab680fc65a"
	windThreeHash = "a47195960e1a4a2606158b3d2978ec0dc911b18aeade2014a2634dad3c40255d"
)

func TestWindImagesResolveTurnBeforeFireWithReassignedIDs(t *testing.T) {
	for _, tt := range []struct {
		name     string
		positive bool
		display  []int8
		want     *int32
	}{
		{"negative", false, []int8{42, -7, 19}, value.Of(int32(-32))},
		{"positive", true, []int8{42, -7, 19}, value.Of(int32(32))},
		{"zero", false, []int8{8, -7, 8}, value.Of(int32(0))},
		{"fraction", true, []int8{8, -7, 19}, value.Of(int32(2))},
		{"missing image", true, []int8{1, -7, 19}, nil},
		{"wrong separator", true, []int8{42, 8, 19}, nil},
		{"separator as digit", true, []int8{-7, -7, 19}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			session := game.NewSession(1, false)
			projector := NewProjector(session)

			for id, hash := range map[int8]string{
				42: windThreeHash,
				-7: windDotHash,
				19: windTwoHash,
				8:  windZeroHash,
			} {
				applyWindPacket(t, projector, windPictureFields(id, hash))
			}

			applyWindPacket(t, projector, windTurnFields(tt.positive, tt.display))

			direction := int32(-1)
			if tt.positive {
				direction = 1
			}

			want := fight.Wind{
				Tenths: tt.want, Current: tt.want != nil,
				Direction: &direction, Display: &[3]int8{tt.display[0], tt.display[1], tt.display[2]},
				Pictures: map[int8]fight.WindPicture{
					42: {SHA256: windThreeHash, Glyph: "3"}, -7: {SHA256: windDotHash, Glyph: "."},
					19: {SHA256: windTwoHash, Glyph: "2"}, 8: {SHA256: windZeroHash, Glyph: "0"},
				},
			}
			if tt.want != nil {
				want.Source = value.Of("display")
			}

			if got := session.Snapshot().Fight.Wind; !reflect.DeepEqual(got, want) {
				t.Fatalf("wind = %#v, want %#v", got, want)
			}
		})
	}
}

func TestWindLateAndReplacedImages(t *testing.T) {
	session := game.NewSession(1, false)
	projector := NewProjector(session)
	applyWindPacket(t, projector, windTurnFields(false, []int8{42, -7, 19}))

	for _, fields := range []map[string]any{windPictureFields(42, windThreeHash), windPictureFields(-7, windDotHash)} {
		applyWindPacket(t, projector, fields)
	}

	assertWindValue(t, session, nil, nil, false)
	applyWindPacket(t, projector, windPictureFields(19, windTwoHash))
	assertWindValue(t, session, value.Of(int32(-32)), value.Of("display"), true)

	applyWindPacket(t, projector, windPictureFields(19, "unrecognized image"))
	assertWindValue(t, session, nil, nil, false)
	if got := session.Snapshot().Fight.Wind.Pictures[19]; got != (fight.WindPicture{SHA256: "unrecognized image"}) {
		t.Fatalf("unknown image = %#v", got)
	}

	applyWindPacket(t, projector, windPictureFields(19, windZeroHash))
	assertWindValue(t, session, value.Of(int32(-30)), value.Of("display"), true)

	shot := windTurnFields(false, []int8{42, -7, 19})
	shot["subcommand"] = uint8(2)
	shot["wind_tenths"] = int32(-29)
	shot["projectiles"] = []map[string]any{}
	shot["pet_targets"] = []map[string]any{}
	applyWindPacket(t, projector, shot)
	assertWindValue(t, session, value.Of(int32(-29)), value.Of("fire"), true)
	applyWindPacket(t, projector, windPictureFields(19, windTwoHash))
	assertWindValue(t, session, value.Of(int32(-29)), value.Of("fire"), true)

	applyWindPacket(t, projector, windTurnFields(true, []int8{19, -7, 42}))
	assertWindValue(t, session, value.Of(int32(23)), value.Of("display"), true)
	applyWindPacket(t, projector, windTurnFields(true, []int8{1, 0, 2}))
	assertWindValue(t, session, nil, nil, false)
}

func TestWindExpiresAndImagesCannotRestoreEndedOrUnknownTurn(t *testing.T) {
	for _, tt := range []struct {
		name   string
		sub    uint8
		status string
	}{
		{"partial turn", 6, "partial"}, {"invalid turn", 6, "invalid"},
		{"mission end", 112, "decoded"}, {"fight end", 115, "decoded"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			session := game.NewSession(1, false)
			projector := NewProjector(session)
			applyWindPacket(t, projector, windPictureFields(8, windZeroHash))
			applyWindPacket(t, projector, windPictureFields(-7, windDotHash))
			applyWindPacket(t, projector, windTurnFields(true, []int8{8, -7, 8}))
			assertWindValue(t, session, value.Of(int32(0)), value.Of("display"), true)
			fields := map[string]any{"subcommand": tt.sub}
			if tt.sub == 112 {
				fields["take_card_type"] = int32(0)
				fields["has_next_mission"] = false
				fields["can_enter_final"] = false
				fields["players"] = []map[string]any{}
			}

			if _, err := projector.Apply("ServerToClient", Header{Code: 91}, Decoded{Status: tt.status, Fields: fields}); err != nil {
				t.Fatal(err)
			}

			applyWindPacket(t, projector, windPictureFields(8, windZeroHash))
			assertWindValue(t, session, nil, nil, false)
		})
	}
}

func TestWindImagesAreDetachedAndSessionScoped(t *testing.T) {
	first, second := game.NewSession(1, false), game.NewSession(2, false)
	projector := NewProjector(first)
	applyWindPacket(t, projector, windPictureFields(8, windZeroHash))
	applyWindPacket(t, projector, windPictureFields(-7, windDotHash))
	applyWindPacket(t, projector, windTurnFields(true, []int8{8, -7, 8}))
	want := first.Snapshot().Fight.Wind
	copy := first.Snapshot()
	copy.Fight.Wind.Pictures[8] = fight.WindPicture{Glyph: "9"}
	*copy.Fight.Wind.Direction = -1
	*copy.Fight.Wind.Source = "changed"
	if got := first.Snapshot().Fight.Wind; !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot mutation changed wind: %#v, want %#v", got, want)
	}

	applyWindPacket(t, projector, windTurnFields(true, []int8{8, -7, 8}))
	assertWindValue(t, first, value.Of(int32(0)), value.Of("display"), true)
	applyWindPacket(t, NewProjector(second), windTurnFields(true, []int8{8, -7, 8}))
	assertWindValue(t, second, nil, nil, false)
	if len(second.Snapshot().Fight.Wind.Pictures) != 0 {
		t.Fatal("wind images leaked across sessions")
	}

	first.Invalidate("gap")
	assertWindValue(t, first, nil, nil, false)
}

func applyWindPacket(t *testing.T, projector *Projector, fields map[string]any) {
	t.Helper()
	if _, err := projector.Apply("ServerToClient", Header{Code: 91}, Decoded{Status: "decoded", Fields: fields}); err != nil {
		t.Fatal(err)
	}
}

func windPictureFields(id int8, hash string) map[string]any {
	glyphs := map[string]string{
		windZeroHash:  "0",
		windDotHash:   ".",
		windTwoHash:   "2",
		windThreeHash: "3",
	}

	return map[string]any{
		"subcommand":     uint8(241),
		"picture_id":     id,
		"picture_sha256": hash,
		"picture_glyph":  glyphs[hash],
	}
}

func windTurnFields(positive bool, display []int8) map[string]any {
	return map[string]any{
		"subcommand":         uint8(6),
		"wind_display_flag":  positive,
		"wind_display_bytes": display,
		"turn_number":        int32(1),
		"turn_time":          int32(20),
		"livings":            []map[string]any{},
	}
}

func assertWindValue(t *testing.T, session *game.Session, tenths *int32, source *string, current bool) {
	t.Helper()
	w := session.Snapshot().Fight.Wind
	if !reflect.DeepEqual(w.Tenths, tenths) || !reflect.DeepEqual(w.Source, source) || w.Current != current {
		t.Fatalf("wind = %#v, want tenths=%v source=%v current=%v", w, tenths, source, current)
	}
}

func TestWindPictureRejectsIntegerNarrowing(t *testing.T) {
	for _, id := range []int64{-129, 128, 2147483648} {
		session := game.NewSession(1, false)
		projector := NewProjector(session)
		before := session.Snapshot()
		_, err := projector.Apply("ServerToClient", Header{Code: 91}, Decoded{Status: "decoded", Fields: map[string]any{
			"subcommand":     int64(241),
			"picture_id":     id,
			"picture_sha256": "test",
			"picture_glyph":  "0",
		}})
		if err == nil || !reflect.DeepEqual(before, session.Snapshot()) {
			t.Fatalf("id=%d error=%v state changed=%t", id, err, !reflect.DeepEqual(before, session.Snapshot()))
		}
	}
}
