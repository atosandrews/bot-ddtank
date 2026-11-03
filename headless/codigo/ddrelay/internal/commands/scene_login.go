package commands

type SceneLogin struct {
	SceneID int32
}

func (SceneLogin) Name() string { return "scene_login" }
