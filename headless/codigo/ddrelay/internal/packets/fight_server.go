package packets

import (
	"crypto/sha256"
	"fmt"

	"ddbot/relay/internal/framing"
)

func fightServer(r *reader, h framing.Header, f map[string]any) error {
	sub := fightCommand(r, f)
	switch sub {
	case 4:
		return r.err
	case 2:
		f["living_id"] = h.Parameter1
		return fightShot(r, f)
	case 6:
		f["living_id"] = h.Parameter1
		return fightTurn(r, f)
	case 7, 14:
		f["living_id"] = h.Parameter1
		name := "direction"
		if sub == 14 {
			name = "dander"
		}
		f[name] = r.i32(name)
	case 11:
		f["living_id"] = h.Parameter1
		f["change_type"] = r.i8("change_type")
		fightInts(r, f, "hp", "effect_amount")
	case 80:
		f["living_id"] = r.i32("living_id")
		value := r.i32("visibility_value")
		f["visibility_value"] = value
		f["visible"] = value != 0
	case 116:
		f["player_id"] = h.ClientID
		f["ready"] = r.boolean("ready")
	case 16:
		fightInts(r, f, "progress", "zone_id", "player_id")
	case 20:
		f["living_id"] = h.Parameter1
		f["special"] = r.boolean("special")
		f["ball_id"] = r.i32("ball_id")
	case 32:
		f["living_id"] = h.Parameter1
		f["bag_type"] = r.i8("bag_type")
		fightInts(r, f, "place", "template_id", "target_id")
		f["show_message"] = r.boolean("show_message")
	case 46:
		f["living_id"] = h.Parameter1
		f["shoot_count"] = r.i8("shoot_count")
	case 41:
		f["living_id"] = h.Parameter1
		fightStrings(r, f, "property", "value")
	case 55:
		f["living_id"] = h.Parameter1
		fightInts(r, f, "from_x", "from_y", "to_x", "to_y", "speed")
		fightStrings(r, f, "action", "end_action")
	case 56:
		f["living_id"] = h.Parameter1
		fightInts(r, f, "x", "y", "speed")
		f["action"] = r.utf("action")
		f["fall_argument"] = r.i32("fall_argument")
	case 58:
		f["living_id"] = h.Parameter1
		f["action"] = r.utf("action")
		f["targets"] = fightEntries(r, "targets", 20, func(e map[string]any) {
			fightInts(r, e, "target_id", "damage", "hp", "dander", "attack_effect")
		})
	case 59:
		f["living_id"] = h.Parameter1
		f["text"] = r.utf("text")
		f["say_type"] = r.i32("say_type")
	case 64:
		return fightLiving(r, f)
	case 67:
		f["resources"] = fightResources(r)
	case 96:
		f["player_id"] = h.ClientID
		started := r.boolean("started")
		f["started"] = started
		if started {
			f["shot_type"] = r.i8("shot_type")
		}
	case 99:
		return fightStart(r, f)
	case 98:
		f["living_id"] = h.Parameter1
		f["player_id"] = h.ClientID
		return fightCardResult(r, f)
	case 89:
		f["cards"] = fightEntries(r, "cards", 9, func(card map[string]any) {
			card["card_index"] = r.i8("card_index")
			fightInts(r, card, "template_id", "count")
		})
	case 101:
		return fightCreate(r, f)
	case 103:
		fightInts(r, f, "max_time", "map_id")
		f["resources"] = fightResources(r)
		f["pet_resources"] = fightEntries(r, "pet_resources", 4, func(e map[string]any) {
			fightStrings(r, e, "picture", "effect")
		})
	case 104:
		fightInts(r, f, "current_value_1", "current_value_2", "current_value_3", "current_value_4")
	case 112:
		return fightMissionOver(r, f)
	case 113:
		f["mission_id"] = r.i32("mission_id")
		fightStrings(r, f, "name", "success", "failure", "description", "mission_config")
		fightInts(r, f, "total_missions", "mission_index", "total_value_1", "total_value_2", "total_value_3", "total_value_4", "try_again")
		f["picture"] = r.utf("picture")
	case 115:
		return fightAllOver(r, f)
	case 131:
		f["lifetime"] = r.i32("lifetime")
	case 134:
		return fightStyle(r, f)
	case 149:
		count := r.i32("count")
		f["count"] = count
		if count == -1 {
			f["self_enabled"] = r.boolean("self_enabled")
			return r.err
		}
		if count < 0 || count > maxEntries || int(count) > (len(r.data)-r.offset)/5 {
			r.fail("count", ErrInvalid)
			return r.err
		}
		entries := make([]map[string]any, 0, count)
		for i := int32(0); i < count && r.err == nil; i++ {
			entries = append(entries, map[string]any{"living_id": r.i32("living_id"), "enabled": r.boolean("enabled")})
		}
		f["players"] = entries
	case 241:
		f["picture_id"] = r.i8("picture_id")
		image := r.take("picture", len(r.data)-r.offset)
		f["picture_bytes"] = len(image)
		f["picture_sha256"] = fmt.Sprintf("%x", sha256.Sum256(image))
		decodeWindPicture(image, f)
	case 252:
		fightInts(r, f, "psychic", "living_id")
	default:
		return ErrUnsupported
	}
	return r.err
}
