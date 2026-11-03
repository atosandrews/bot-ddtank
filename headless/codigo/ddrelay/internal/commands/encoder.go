package commands

import (
	"fmt"

	"ddbot/api/contract"
	"ddbot/relay/internal/framing"
)

func Encode(command Command) (framing.Frame, error) {
	w := &writer{}
	code := int16(91)

	switch c := command.(type) {
	case OrganizeInventory:
		if err := contract.ValidateInventoryOrganization(c.BagType, c.Places); err != nil {
			return framing.Frame{}, &EncodingError{Field: "organize_inventory", Reason: err.Error()}
		}

		code = 124
		start := int32(0)
		if c.BagType == 0 {
			start = 31
		}

		w.flag(true)
		w.ints(int32(len(c.Places)), c.BagType)
		for index, place := range c.Places {
			w.ints(place, start+int32(index))
		}
	case CollectTemporaryRewards:
		code = 108
		w.ints(-1)
	case UnlockBag:
		if len(c.Password) == 0 || len(c.Password) > 64 {
			return framing.Frame{}, &EncodingError{Field: "password", Reason: "password must contain 1 to 64 UTF-8 bytes"}
		}

		code = 25
		w.text("password", c.Password)
		w.text("new_password", "")
		w.ints(2)
		w.text("question_1", "")
		w.text("answer_1", "")
		w.text("question_2", "")
		w.text("answer_2", "")
	case CaptchaAnswer:
		if len(c.Text) == 0 || len(c.Text) > 64 {
			return framing.Frame{}, &EncodingError{Field: "text", Reason: "answer must contain 1 to 64 UTF-8 bytes"}
		}

		code = 200
		w.text("answer", c.Text)
	case FriendAdd:
		if c.NameValue == "" {
			return framing.Frame{}, &EncodingError{Field: "name", Reason: "name required"}
		}
		code = 160
		w.byte(160)
		w.text("name", c.NameValue)
		w.ints(c.Relation)
		w.flag(c.Flag1)
		w.flag(c.Flag2)
	case RoomInvite:
		if c.PlayerID <= 0 {
			return framing.Frame{}, &EncodingError{Field: "player_id", Reason: "positive ID required"}
		}
		code = 70
		w.ints(c.PlayerID)
	case CardTake:
		if c.Index < 0 || c.Index > 20 {
			return framing.Frame{}, &EncodingError{Field: "index", Reason: "card slot outside supported board"}
		}
		w.byte(98)
		w.int8("index", c.Index)
	case CardTakeBoss:
		if c.Index < 0 || c.Index > 8 {
			return framing.Frame{}, &EncodingError{Field: "index", Reason: "card slot outside supported board"}
		}
		w.byte(130)
		w.int8("index", c.Index)
	case CardTakeExtra:
		if c.Index < 0 || c.Index > 20 {
			return framing.Frame{}, &EncodingError{Field: "index", Reason: "card slot outside supported board"}
		}
		w.byte(114)
		w.int8("index", c.Index)
	case CardFinish:
		w.byte(98)
		w.byte(100)
	case CardFinishBoss:
		w.byte(130)
		w.byte(100)
	case SceneLogin:
		code = 16
		w.ints(c.SceneID)
	case SceneState:
		code = 251
		w.ints(c.State)
	case Shoot:
		w.byte(2)
		w.ints(c.X, c.Y, c.Force, c.Angle)
	case Direction:
		w.byte(7)
		w.ints(c.Direction)
	case MoveStart:
		w.byte(9)
		w.flag(c.Moving)
		w.int8("move_type", c.MoveType)
		w.ints(c.X, c.Y)
		w.int8("direction", c.Direction)
		w.flag(c.Living)
		w.int16("turn_number", c.TurnNumber)
	case MoveStop:
		w.byte(10)
		w.ints(c.X, c.Y)
		w.flag(c.MoveFlag)
	case SkipTurn:
		w.byte(12)
		w.int8("argument", c.Argument)
	case Pow:
		w.byte(15)
		w.byte(0)
	case UseProp:
		w.byte(32)
		w.int8("bag_type", c.BagType)
		w.ints(c.Place, c.TemplateID)
	case LoadProgress:
		w.byte(16)
		w.ints(c.Progress)
	case ShootTag:
		w.byte(96)
		w.flag(c.Started)
		if c.Started {
			w.int8("shot_type", c.ShotType)
		}
	case MissionReady:
		w.byte(116)
		w.flag(c.Ready)
	case MissionStart:
		code = 82
		w.flag(c.Flag)
	case RoomCreate:
		code = 94
		w.ints(0)
		w.int8("room_type", c.RoomType)
		w.int8("time_type", c.TimeType)
		w.text("name", c.NameValue)
		w.text("password", c.Password)
	case RoomJoin:
		code = 94
		w.ints(1)
		w.flag(c.LoginFlag)
		w.ints(c.RoomType, c.RoomID)
		if c.RoomID == -1 {
			w.ints(c.Argument)
			w.text("password", c.Password)
		}
	case RoomSetup:
		code = 94
		w.ints(2, c.MapID)
		w.int8("room_type", c.RoomType)
		w.text("password", c.Password)
		w.text("name", c.NameValue)
		w.int8("time_type", c.TimeType)
		w.int8("difficulty", c.Difficulty)
		w.ints(c.LevelLimits)
		w.flag(c.Flag1)
		w.flag(c.Flag2)
	case RoomLeave:
		code = 94
		w.ints(5)
	case RoomStart:
		code = 94
		w.ints(7)
	case RoomReady:
		code = 94
		w.ints(15)
		w.int8("state", c.State)
	default:
		return framing.Frame{}, &EncodingError{Field: "command", Reason: fmt.Sprintf("unsupported command type %T", command)}
	}

	if w.err != nil {
		return framing.Frame{}, w.err
	}

	return framing.Frame{Header: framing.Header{Code: code, Length: uint16(framing.HeaderSize + len(w.data))}, Body: w.data}, nil
}
