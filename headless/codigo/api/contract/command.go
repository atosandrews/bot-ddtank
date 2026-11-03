package contract

import (
	"math"
	"unicode/utf8"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func ValidateCommand(command *pb.Command) error {
	if command == nil {
		return &Error{Field: "command", Reason: "command required"}
	}

	if err := rejectUnknown(command.ProtoReflect(), 0); err != nil {
		return err
	}

	switch v := command.Payload.(type) {
	case *pb.Command_CaptchaAnswer:
		if v.CaptchaAnswer == nil {
			return &Error{Field: "captcha_answer", Reason: "payload required"}
		}

		if v.CaptchaAnswer.Revision == 0 {
			return &Error{Field: "revision", Reason: "challenge revision required"}
		}

		if len(v.CaptchaAnswer.Text) == 0 || len(v.CaptchaAnswer.Text) > 64 || !utf8.ValidString(v.CaptchaAnswer.Text) {
			return &Error{Field: "text", Reason: "answer must contain 1 to 64 UTF-8 bytes"}
		}
	case *pb.Command_Direction:
		if v.Direction == nil {
			return &Error{Field: "direction", Reason: "payload required"}
		}
	case *pb.Command_LoadProgress:
		if v.LoadProgress == nil {
			return &Error{Field: "load_progress", Reason: "payload required"}
		}
	case *pb.Command_MissionReady:
		if v.MissionReady == nil {
			return &Error{Field: "mission_ready", Reason: "payload required"}
		}
	case *pb.Command_MissionStart:
		if v.MissionStart == nil {
			return &Error{Field: "mission_start", Reason: "payload required"}
		}
	case *pb.Command_MoveStart:
		if v.MoveStart == nil {
			return &Error{Field: "move_start", Reason: "payload required"}
		}
		if v.MoveStart.MoveType < math.MinInt8 || v.MoveStart.MoveType > math.MaxInt8 {
			return &Error{Field: "move_type", Reason: "integer exceeds wire range"}
		}
		if v.MoveStart.Direction < math.MinInt8 || v.MoveStart.Direction > math.MaxInt8 {
			return &Error{Field: "direction", Reason: "integer exceeds wire range"}
		}
		if v.MoveStart.TurnNumber < math.MinInt16 || v.MoveStart.TurnNumber > math.MaxInt16 {
			return &Error{Field: "turn_number", Reason: "integer exceeds wire range"}
		}
	case *pb.Command_MoveStop:
		if v.MoveStop == nil {
			return &Error{Field: "move_stop", Reason: "payload required"}
		}
	case *pb.Command_RoomCreate:
		if v.RoomCreate == nil {
			return &Error{Field: "room_create", Reason: "payload required"}
		}
		if v.RoomCreate.RoomType < math.MinInt8 || v.RoomCreate.RoomType > math.MaxInt8 {
			return &Error{Field: "room_type", Reason: "integer exceeds wire range"}
		}
		if v.RoomCreate.TimeType < math.MinInt8 || v.RoomCreate.TimeType > math.MaxInt8 {
			return &Error{Field: "time_type", Reason: "integer exceeds wire range"}
		}
		if len(v.RoomCreate.NameValue) > 65535 || !utf8.ValidString(v.RoomCreate.NameValue) {
			return &Error{Field: "name_value", Reason: "invalid UTF-8 or string length"}
		}
		if len(v.RoomCreate.Password) > 65535 || !utf8.ValidString(v.RoomCreate.Password) {
			return &Error{Field: "password", Reason: "invalid UTF-8 or string length"}
		}
	case *pb.Command_RoomJoin:
		if v.RoomJoin == nil {
			return &Error{Field: "room_join", Reason: "payload required"}
		}
		if len(v.RoomJoin.Password) > 65535 || !utf8.ValidString(v.RoomJoin.Password) {
			return &Error{Field: "password", Reason: "invalid UTF-8 or string length"}
		}
	case *pb.Command_RoomLeave:
		if v.RoomLeave == nil {
			return &Error{Field: "room_leave", Reason: "payload required"}
		}
	case *pb.Command_RoomReady:
		if v.RoomReady == nil {
			return &Error{Field: "room_ready", Reason: "payload required"}
		}
		if v.RoomReady.State < math.MinInt8 || v.RoomReady.State > math.MaxInt8 {
			return &Error{Field: "state", Reason: "integer exceeds wire range"}
		}
	case *pb.Command_RoomSetup:
		if v.RoomSetup == nil {
			return &Error{Field: "room_setup", Reason: "payload required"}
		}
		if v.RoomSetup.RoomType < math.MinInt8 || v.RoomSetup.RoomType > math.MaxInt8 {
			return &Error{Field: "room_type", Reason: "integer exceeds wire range"}
		}
		if len(v.RoomSetup.Password) > 65535 || !utf8.ValidString(v.RoomSetup.Password) {
			return &Error{Field: "password", Reason: "invalid UTF-8 or string length"}
		}
		if len(v.RoomSetup.NameValue) > 65535 || !utf8.ValidString(v.RoomSetup.NameValue) {
			return &Error{Field: "name_value", Reason: "invalid UTF-8 or string length"}
		}
		if v.RoomSetup.TimeType < math.MinInt8 || v.RoomSetup.TimeType > math.MaxInt8 {
			return &Error{Field: "time_type", Reason: "integer exceeds wire range"}
		}
		if v.RoomSetup.Difficulty < math.MinInt8 || v.RoomSetup.Difficulty > math.MaxInt8 {
			return &Error{Field: "difficulty", Reason: "integer exceeds wire range"}
		}
	case *pb.Command_RoomStart:
		if v.RoomStart == nil {
			return &Error{Field: "room_start", Reason: "payload required"}
		}
	case *pb.Command_SceneLogin:
		if v.SceneLogin == nil {
			return &Error{Field: "scene_login", Reason: "payload required"}
		}
	case *pb.Command_SceneState:
		if v.SceneState == nil {
			return &Error{Field: "scene_state", Reason: "payload required"}
		}
	case *pb.Command_Shoot:
		if v.Shoot == nil {
			return &Error{Field: "shoot", Reason: "payload required"}
		}
	case *pb.Command_ShootTag:
		if v.ShootTag == nil {
			return &Error{Field: "shoot_tag", Reason: "payload required"}
		}
		if v.ShootTag.ShotType < math.MinInt8 || v.ShootTag.ShotType > math.MaxInt8 {
			return &Error{Field: "shot_type", Reason: "integer exceeds wire range"}
		}
	case *pb.Command_SkipTurn:
		if v.SkipTurn == nil {
			return &Error{Field: "skip_turn", Reason: "payload required"}
		}
		if v.SkipTurn.Argument < math.MinInt8 || v.SkipTurn.Argument > math.MaxInt8 {
			return &Error{Field: "argument", Reason: "integer exceeds wire range"}
		}
	case *pb.Command_Pow:
		if v.Pow == nil {
			return &Error{Field: "pow", Reason: "payload required"}
		}
	case *pb.Command_UseProp:
		if v.UseProp == nil {
			return &Error{Field: "use_prop", Reason: "payload required"}
		}
		if v.UseProp.BagType < math.MinInt8 || v.UseProp.BagType > math.MaxInt8 {
			return &Error{Field: "bag_type", Reason: "integer exceeds wire range"}
		}
	case *pb.Command_FriendAdd:
		if v.FriendAdd == nil {
			return &Error{Field: "friend_add", Reason: "payload required"}
		}
		if len(v.FriendAdd.Name) == 0 || len(v.FriendAdd.Name) > 65506 || !utf8.ValidString(v.FriendAdd.Name) {
			return &Error{Field: "name", Reason: "invalid UTF-8 or name length"}
		}
	case *pb.Command_RoomInvite:
		if v.RoomInvite == nil {
			return &Error{Field: "room_invite", Reason: "payload required"}
		}
		if v.RoomInvite.PlayerId <= 0 {
			return &Error{Field: "player_id", Reason: "positive player ID required"}
		}
	case *pb.Command_CardTake:
		if v.CardTake == nil {
			return &Error{Field: "card_take", Reason: "payload required"}
		}
		if v.CardTake.Index < 0 || v.CardTake.Index > 20 {
			return &Error{Field: "index", Reason: "card slot outside supported board"}
		}
	case *pb.Command_CardTakeBoss:
		if v.CardTakeBoss == nil {
			return &Error{Field: "card_take_boss", Reason: "payload required"}
		}
		if v.CardTakeBoss.Index < 0 || v.CardTakeBoss.Index > 8 {
			return &Error{Field: "index", Reason: "card slot outside supported board"}
		}
	case *pb.Command_CardTakeExtra:
		if v.CardTakeExtra == nil {
			return &Error{Field: "card_take_extra", Reason: "payload required"}
		}
		if v.CardTakeExtra.Index < 0 || v.CardTakeExtra.Index > 20 {
			return &Error{Field: "index", Reason: "card slot outside supported board"}
		}
	case *pb.Command_CardFinish:
		if v.CardFinish == nil {
			return &Error{Field: "card_finish", Reason: "payload required"}
		}
	case *pb.Command_CollectTemporaryRewards:
		if v.CollectTemporaryRewards == nil {
			return &Error{Field: "collect_temporary_rewards", Reason: "payload required"}
		}
	case *pb.Command_OrganizeInventory:
		if v.OrganizeInventory == nil {
			return &Error{Field: "organize_inventory", Reason: "payload required"}
		}

		return ValidateInventoryOrganization(v.OrganizeInventory.BagType, v.OrganizeInventory.Places)
	case *pb.Command_UnlockBag:
		if v.UnlockBag == nil {
			return &Error{Field: "unlock_bag", Reason: "payload required"}
		}
		if len(v.UnlockBag.Password) == 0 || len(v.UnlockBag.Password) > 64 || !utf8.ValidString(v.UnlockBag.Password) {
			return &Error{Field: "password", Reason: "invalid UTF-8 or password length"}
		}
	case *pb.Command_CardFinishBoss:
		if v.CardFinishBoss == nil {
			return &Error{Field: "card_finish_boss", Reason: "payload required"}
		}
	default:
		return &Error{Field: "command", Reason: "unsupported or missing command variant"}
	}

	return nil
}
