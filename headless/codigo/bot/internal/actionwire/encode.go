package actionwire

import (
	"fmt"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/game/action"
)

func Encode(id uint64, command action.Command) (*pb.Command, error) {
	result := &pb.Command{Id: id}

	switch c := command.(type) {
	case action.OrganizeInventory:
		result.Payload = &pb.Command_OrganizeInventory{OrganizeInventory: &pb.OrganizeInventory{BagType: c.BagType, Places: c.Places}}
	case action.CollectTemporaryRewards:
		result.Payload = &pb.Command_CollectTemporaryRewards{CollectTemporaryRewards: &pb.CollectTemporaryRewards{}}
	case action.UnlockBag:
		result.Payload = &pb.Command_UnlockBag{UnlockBag: &pb.UnlockBag{Password: c.Password}}
	case action.CaptchaAnswer:
		result.Payload = &pb.Command_CaptchaAnswer{CaptchaAnswer: &pb.CaptchaAnswer{Revision: c.Revision, Text: c.Text}}
	case action.FriendAdd:
		result.Payload = &pb.Command_FriendAdd{FriendAdd: &pb.FriendAdd{
			Name:     c.NameValue,
			Relation: c.Relation,
			Flag1:    c.Flag1,
			Flag2:    c.Flag2,
		}}
	case action.RoomInvite:
		result.Payload = &pb.Command_RoomInvite{RoomInvite: &pb.RoomInvite{PlayerId: c.PlayerID}}
	case action.CardTake:
		result.Payload = &pb.Command_CardTake{CardTake: &pb.CardTake{Index: c.Index}}
	case action.CardTakeBoss:
		result.Payload = &pb.Command_CardTakeBoss{CardTakeBoss: &pb.CardTakeBoss{Index: c.Index}}
	case action.CardTakeExtra:
		result.Payload = &pb.Command_CardTakeExtra{CardTakeExtra: &pb.CardTakeExtra{Index: c.Index}}
	case action.CardFinish:
		result.Payload = &pb.Command_CardFinish{CardFinish: &pb.CardFinish{}}
	case action.CardFinishBoss:
		result.Payload = &pb.Command_CardFinishBoss{CardFinishBoss: &pb.CardFinishBoss{}}
	case action.Direction:
		result.Payload = &pb.Command_Direction{Direction: &pb.Direction{Direction: c.Direction}}
	case action.LoadProgress:
		result.Payload = &pb.Command_LoadProgress{LoadProgress: &pb.LoadProgress{Progress: c.Progress}}
	case action.MissionReady:
		result.Payload = &pb.Command_MissionReady{MissionReady: &pb.MissionReady{Ready: c.Ready}}
	case action.MissionStart:
		result.Payload = &pb.Command_MissionStart{MissionStart: &pb.MissionStart{Flag: c.Flag}}
	case action.MoveStart:
		result.Payload = &pb.Command_MoveStart{MoveStart: &pb.MoveStart{
			Moving:     c.Moving,
			MoveType:   int32(c.MoveType),
			X:          c.X,
			Y:          c.Y,
			Direction:  int32(c.Direction),
			Living:     c.Living,
			TurnNumber: int32(c.TurnNumber),
		}}
	case action.MoveStop:
		result.Payload = &pb.Command_MoveStop{MoveStop: &pb.MoveStop{X: c.X, Y: c.Y, MoveFlag: c.MoveFlag}}
	case action.RoomCreate:
		result.Payload = &pb.Command_RoomCreate{RoomCreate: &pb.RoomCreate{
			RoomType:  int32(c.RoomType),
			TimeType:  int32(c.TimeType),
			NameValue: c.NameValue,
			Password:  c.Password,
		}}
	case action.RoomJoin:
		result.Payload = &pb.Command_RoomJoin{RoomJoin: &pb.RoomJoin{
			LoginFlag: c.LoginFlag,
			RoomType:  c.RoomType,
			RoomId:    c.RoomID,
			Argument:  c.Argument,
			Password:  c.Password,
		}}
	case action.RoomLeave:
		result.Payload = &pb.Command_RoomLeave{RoomLeave: &pb.RoomLeave{}}
	case action.RoomReady:
		result.Payload = &pb.Command_RoomReady{RoomReady: &pb.RoomReady{State: int32(c.State)}}
	case action.RoomSetup:
		result.Payload = &pb.Command_RoomSetup{RoomSetup: &pb.RoomSetup{
			MapId:       c.MapID,
			RoomType:    int32(c.RoomType),
			Password:    c.Password,
			NameValue:   c.NameValue,
			TimeType:    int32(c.TimeType),
			Difficulty:  int32(c.Difficulty),
			LevelLimits: c.LevelLimits,
			Flag1:       c.Flag1,
			Flag2:       c.Flag2,
		}}
	case action.RoomStart:
		result.Payload = &pb.Command_RoomStart{RoomStart: &pb.RoomStart{}}
	case action.SceneLogin:
		result.Payload = &pb.Command_SceneLogin{SceneLogin: &pb.SceneLogin{SceneId: c.SceneID}}
	case action.SceneState:
		result.Payload = &pb.Command_SceneState{SceneState: &pb.SceneState{State: c.State}}
	case action.Shoot:
		result.Payload = &pb.Command_Shoot{Shoot: &pb.Shoot{
			X:     c.X,
			Y:     c.Y,
			Force: c.Force,
			Angle: c.Angle,
		}}
	case action.ShootTag:
		result.Payload = &pb.Command_ShootTag{ShootTag: &pb.ShootTag{Started: c.Started, ShotType: int32(c.ShotType)}}
	case action.SkipTurn:
		result.Payload = &pb.Command_SkipTurn{SkipTurn: &pb.SkipTurn{Argument: int32(c.Argument)}}
	case action.Pow:
		result.Payload = &pb.Command_Pow{Pow: &pb.Pow{}}
	case action.UseProp:
		result.Payload = &pb.Command_UseProp{UseProp: &pb.UseProp{BagType: int32(c.BagType), Place: c.Place, TemplateId: c.TemplateID}}
	default:
		return nil, &action.InvalidCommandError{Reason: fmt.Sprintf("unsupported command type %T", command)}
	}

	if err := contract.ValidateCommand(result); err != nil {
		return nil, &action.InvalidCommandError{Name: command.Name(), Reason: err.Error(), Cause: err}
	}

	return result, nil
}
