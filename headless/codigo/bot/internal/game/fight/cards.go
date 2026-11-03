package fight

import "maps"

const CardsNone int32 = 0
const CardsSmall int32 = 1
const CardsLarge int32 = 2

type Cards struct {
	Revision        uint64
	Known           bool
	Type            int32
	HasNextMission  bool
	CanEnterFinal   bool
	Players         map[int32]CardPlayer
	Awards          map[int32]CardAward
	Revealed        map[int32]CardReveal
	Requested       map[int32]string
	Finished        bool
	Closed          bool
	SystemSelection bool
}

func (c Cards) Clone() Cards {
	c.Players = maps.Clone(c.Players)
	c.Awards = maps.Clone(c.Awards)

	for index, award := range c.Awards {
		c.Awards[index] = award.Clone()
	}

	c.Revealed = maps.Clone(c.Revealed)
	c.Requested = maps.Clone(c.Requested)

	return c
}

func (c Cards) Reset() Cards { return Cards{Revision: c.Revision + 1} }
