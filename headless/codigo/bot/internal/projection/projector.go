package projection

import (
	"errors"

	"ddbot/bot/internal/game"
	"ddbot/bot/internal/game/captcha"
	"ddbot/bot/internal/game/fight"
	"ddbot/bot/internal/game/player"
	"ddbot/bot/internal/game/room"
	"ddbot/bot/internal/game/social"
	"ddbot/bot/internal/game/value"
)

type Projector struct{ session *game.Session }

func NewProjector(session *game.Session) *Projector { return &Projector{session: session} }

func (p *Projector) Apply(direction string, header Header, decoded Decoded) (string, error) {
	if direction == "ClientToServer" {
		if header.Code == 1 {
			next := p.session.Snapshot()
			next.Valid = false
			next.Player = player.State{InventoryRevision: next.Player.InventoryRevision + 1}
			if err := p.session.Commit(next); err != nil {
				return "", err
			}

			return "player_login_started", nil
		}

		if decoded.Status == "decoded" && header.Code == 91 {
			sub := decoded.Fields["subcommand"]
			if integerEqual(sub, 98) || integerEqual(sub, 114) || integerEqual(sub, 130) {
				return p.projectCardRequest(decoded)
			}
		}

		return p.expireClientCombatState(header, decoded)
	}

	if direction != "ServerToClient" {
		return "", nil
	}

	next := p.session.Snapshot()
	if decoded.Status != "decoded" && header.Code == 91 && integerEqual(decoded.Fields["subcommand"], 46) {
		for id, actor := range next.Fight.Actors {
			if next.Player.ID != nil && actor.PlayerID != nil && *actor.PlayerID == *next.Player.ID {
				actor.ShootCount = nil
				next.Fight.Actors[id] = actor
			}
		}
		if err := p.session.Commit(next); err != nil {
			return "", err
		}
		return "shoot_count_unknown", nil
	}

	if decoded.Status != "decoded" && header.Code == 200 {
		next.Captcha = captcha.State{Revision: next.Captcha.Revision + 1, Required: true}
		if err := p.session.Commit(next); err != nil {
			return "", err
		}

		return "captcha_unknown", nil
	}

	if decoded.Status != "decoded" {
		if header.Code == 64 || (header.Code == 185 || header.Code == 186) && next.Player.ID != nil && header.ClientID == *next.Player.ID {
			name := "inventory_unknown"
			if header.Code == 64 {
				next.Player.Inventory = nil
				next.Player.BagRevisions = nil
				next.Player.InventoryUncertain = true
				next.Player.InventoryRevision++
			} else {
				next.Player.Buffs = nil
				next.Player.BuffsKnown = false
				name = "player_buffs_unknown"
			}

			if err := p.session.Commit(next); err != nil {
				return "", err
			}

			return name, nil
		}

		if header.Code == 91 && integerEqual(decoded.Fields["subcommand"], 41) {
			actor := next.Fight.Actors[header.Parameter1]
			actor.ID = header.Parameter1
			actor.Controls.Known = false
			actor.Energy = nil
			actor.EnergyIssue = value.Of("incomplete property update")
			next.Fight.Actors[actor.ID] = actor
			if err := p.session.Commit(next); err != nil {
				return "", err
			}

			return "actor_properties_unknown", nil
		}

		if header.Code == 91 && (integerEqual(decoded.Fields["subcommand"], 112) || integerEqual(decoded.Fields["subcommand"], 103) || integerEqual(decoded.Fields["subcommand"], 101) || integerEqual(decoded.Fields["subcommand"], 98) || integerEqual(decoded.Fields["subcommand"], 89)) {
			next.Fight.Cards = next.Fight.Cards.Reset()
			if err := p.session.Commit(next); err != nil {
				return "", err
			}

			return "cards_unknown", nil
		}

		if header.Code == 91 && integerEqual(decoded.Fields["subcommand"], 6) {
			next.Fight.Turn = nil
			next.Fight.TurnRevision++
			next.Fight.Wind.Expire()
			next.Fight.Wind.Display = nil
			next.Fight.Wind.Direction = nil
			if err := p.session.Commit(next); err != nil {
				return "", err
			}

			return "turn_unknown", nil
		}

		return "", nil
	}

	f := &fields{data: decoded.Fields}
	name, err := project(&next, header, f)
	var unsupported *UnsupportedProjectionError
	if errors.As(err, &unsupported) {
		return "", nil
	}

	if err != nil {
		return "", err
	}

	if f.err != nil {
		return "", f.err
	}

	if name == "" {
		return "", nil
	}

	if err := p.session.Commit(next); err != nil {
		return "", err
	}

	return name, nil
}

func project(next *game.Snapshot, header Header, f *fields) (string, error) {

	switch header.Code {
	case 1:
		if f.integer("status") == 0 {
			if next.Player.ID != nil && *next.Player.ID != header.ClientID {
				next.Player.Inventory = nil
				next.Player.BagRevisions = nil
				next.Player.InventoryUncertain = true
				next.Player.InventoryRevision++
			}

			next.Valid = true
			next.Player.ID = value.Of(header.ClientID)
			next.Player.BagLock = nil
			next.Player.BuffsKnown = false
			next.Player.Buffs = nil
			next.Room = room.NewState()
			next.Social = social.State{}
			next.Captcha = captcha.State{Revision: next.Captcha.Revision + 1}
			next.Message = nil
			generation := next.Fight.Generation + 1
			next.Fight = fight.NewState()
			next.Fight.Generation = generation

			return "player_loaded", nil
		}
	case 200:
		return projectCaptcha(next, f)
	case 25:
		revision := uint64(1)
		if next.Player.BagLock != nil {
			revision = next.Player.BagLock.Revision + 1
		}

		next.Player.BagLock = &player.BagLock{Revision: revision, Operation: f.integer("operation"), Success: f.flag("success"), Message: f.text("message")}
		return "bag_lock_result", nil
	case 3:
		next.Message = &game.Message{Kind: f.integer("kind"), Text: f.text("message")}
		return "server_message", nil
	case 64:
		return projectInventory(next, f)
	case 185, 186:
		return projectPlayerBuffs(next, header, f)
	case 160:
		return projectSocial(next, f)
	case 70:
		return projectInvitation(next, f)
	case 91:
		return projectFight(next, header, f)
	case 94:
		return projectRoom(next, header, f)
	default:
		return "", &UnsupportedProjectionError{}
	}

	return "", nil
}
