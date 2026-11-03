package commands

type FriendAdd struct {
	NameValue    string
	Relation     int32
	Flag1, Flag2 bool
}

func (FriendAdd) Name() string { return "friend_add" }
