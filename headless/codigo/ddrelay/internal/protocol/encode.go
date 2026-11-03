package protocol

import (
	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/relay/internal/commands"
	"ddbot/relay/internal/framing"
)

func Encode(command *pb.Command) (framing.Frame, error) {
	if err := contract.ValidateCommand(command); err != nil {
		return framing.Frame{}, err
	}

	switch v := command.Payload.(type) {
	case *pb.Command_OrganizeInventory:
		return commands.Encode(commands.OrganizeInventory{BagType: v.OrganizeInventory.BagType, Places: v.OrganizeInventory.Places})
	case *pb.Command_CollectTemporaryRewards:
		return commands.Encode(commands.CollectTemporaryRewards{})
	case *pb.Command_UnlockBag:
		return commands.Encode(commands.UnlockBag{Password: v.UnlockBag.Password})
	case *pb.Command_CaptchaAnswer:
		return commands.Encode(commands.CaptchaAnswer{Text: v.CaptchaAnswer.Text})
	case *pb.Command_FriendAdd:
		return commands.Encode(commands.FriendAdd{
			NameValue: v.FriendAdd.Name,
			Relation:  v.FriendAdd.Relation,
			Flag1:     v.FriendAdd.Flag1,
			Flag2:     v.FriendAdd.Flag2,
		})
	case *pb.Command_RoomInvite:
		return commands.Encode(commands.RoomInvite{PlayerID: v.RoomInvite.PlayerId})
	case *pb.Command_CardTake:
		return commands.Encode(commands.CardTake{Index: v.CardTake.Index})
	case *pb.Command_CardTakeBoss:
		return commands.Encode(commands.CardTakeBoss{Index: v.CardTakeBoss.Index})
	case *pb.Command_CardTakeExtra:
		return commands.Encode(commands.CardTakeExtra{Index: v.CardTakeExtra.Index})
	case *pb.Command_CardFinish:
		return commands.Encode(commands.CardFinish{})
	case *pb.Command_CardFinishBoss:
		return commands.Encode(commands.CardFinishBoss{})
	case *pb.Command_Direction:
		return commands.Encode(commands.Direction{Direction: v.Direction.Direction})
	case *pb.Command_LoadProgress:
		return commands.Encode(commands.LoadProgress{Progress: v.LoadProgress.Progress})
	case *pb.Command_MissionReady:
		return commands.Encode(commands.MissionReady{Ready: v.MissionReady.Ready})
	case *pb.Command_MissionStart:
		return commands.Encode(commands.MissionStart{Flag: v.MissionStart.Flag})
	case *pb.Command_MoveStart:
		return commands.Encode(commands.MoveStart{
			Moving:     v.MoveStart.Moving,
			MoveType:   v.MoveStart.MoveType,
			X:          v.MoveStart.X,
			Y:          v.MoveStart.Y,
			Direction:  v.MoveStart.Direction,
			Living:     v.MoveStart.Living,
			TurnNumber: v.MoveStart.TurnNumber,
		})
	case *pb.Command_MoveStop:
		return commands.Encode(commands.MoveStop{X: v.MoveStop.X, Y: v.MoveStop.Y, MoveFlag: v.MoveStop.MoveFlag})
	case *pb.Command_RoomCreate:
		return commands.Encode(commands.RoomCreate{
			RoomType:  v.RoomCreate.RoomType,
			TimeType:  v.RoomCreate.TimeType,
			NameValue: v.RoomCreate.NameValue,
			Password:  v.RoomCreate.Password,
		})
	case *pb.Command_RoomJoin:
		return commands.Encode(commands.RoomJoin{
			LoginFlag: v.RoomJoin.LoginFlag,
			RoomType:  v.RoomJoin.RoomType,
			RoomID:    v.RoomJoin.RoomId,
			Argument:  v.RoomJoin.Argument,
			Password:  v.RoomJoin.Password,
		})
	case *pb.Command_RoomLeave:
		return commands.Encode(commands.RoomLeave{})
	case *pb.Command_RoomReady:
		return commands.Encode(commands.RoomReady{State: v.RoomReady.State})
	case *pb.Command_RoomSetup:
		return commands.Encode(commands.RoomSetup{
			MapID:       v.RoomSetup.MapId,
			RoomType:    v.RoomSetup.RoomType,
			Password:    v.RoomSetup.Password,
			NameValue:   v.RoomSetup.NameValue,
			TimeType:    v.RoomSetup.TimeType,
			Difficulty:  v.RoomSetup.Difficulty,
			LevelLimits: v.RoomSetup.LevelLimits,
			Flag1:       v.RoomSetup.Flag1,
			Flag2:       v.RoomSetup.Flag2,
		})
	case *pb.Command_RoomStart:
		return commands.Encode(commands.RoomStart{})
	case *pb.Command_SceneLogin:
		return commands.Encode(commands.SceneLogin{SceneID: v.SceneLogin.SceneId})
	case *pb.Command_SceneState:
		return commands.Encode(commands.SceneState{State: v.SceneState.State})
	case *pb.Command_Shoot:
		return commands.Encode(commands.Shoot{
			X:     v.Shoot.X,
			Y:     v.Shoot.Y,
			Force: v.Shoot.Force,
			Angle: v.Shoot.Angle,
		})
	case *pb.Command_ShootTag:
		return commands.Encode(commands.ShootTag{Started: v.ShootTag.Started, ShotType: v.ShootTag.ShotType})
	case *pb.Command_SkipTurn:
		return commands.Encode(commands.SkipTurn{Argument: v.SkipTurn.Argument})
	case *pb.Command_Pow:
		return commands.Encode(commands.Pow{})
	case *pb.Command_UseProp:
		return commands.Encode(commands.UseProp{BagType: v.UseProp.BagType, Place: v.UseProp.Place, TemplateID: v.UseProp.TemplateId})
	default:
		return framing.Frame{}, &contract.Error{Field: "command", Reason: "unsupported variant"}
	}
}
