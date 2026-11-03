package action

func RequiresTurn(command Command) (bool, error) {
	switch command.(type) {
	case CollectTemporaryRewards, UnlockBag, OrganizeInventory:
		return false, nil
	case Shoot, Direction, MoveStart, MoveStop, SkipTurn, Pow, UseProp, ShootTag:
		return true, nil
	case CaptchaAnswer, FriendAdd, RoomInvite, CardTake, CardTakeBoss, CardTakeExtra, CardFinish, CardFinishBoss, LoadProgress, MissionReady, MissionStart, RoomCreate, RoomJoin, RoomSetup, RoomLeave, RoomStart, RoomReady, SceneLogin, SceneState:
		return false, nil
	default:
		return false, &InvalidCommandError{Reason: "unsupported command type"}
	}
}
