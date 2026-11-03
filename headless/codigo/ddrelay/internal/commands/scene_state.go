package commands

type SceneState struct {
	State int32
}

func (SceneState) Name() string { return "scene_state" }
