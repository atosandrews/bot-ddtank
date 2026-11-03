package packets

import (
	"crypto/sha256"
	"fmt"
	"os"
	"reflect"
	"testing"

	"ddbot/relay/internal/capture"
	"ddbot/relay/internal/framing"
)

func TestPrivatePremiumBossSequence(t *testing.T) {
	path := os.Getenv("DDTANK_PREMIUM_CAPTURE")
	if path == "" {
		t.Skip("private premium prefix not configured")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	before := sha256.Sum256(raw)
	if fmt.Sprintf("%x", before) != "ef49621c0a503109d6af64d1c26f1a654dd3b2770f87da3d171e96f469039895" {
		t.Fatal("unexpected capture hash")
	}
	framers := map[capture.Direction]*framing.EncryptedFramer{capture.ClientToServer: framing.NewEncryptedFramer(), capture.ServerToClient: framing.NewEncryptedFramer()}
	decoder := NewDecoder()
	var requests, confirmed []int32
	var shots [][]int32
	var cards []uint8
	var boss, damage, hp int32
	var allowance int8
	frames, fires := 0, 0
	active := false
	err = capture.Read(path, func(record capture.Record) error {
		if record.Stream != 2 {
			return nil
		}
		for _, segment := range framers[record.Direction].Push(record.Data) {
			if segment.Kind != framing.FrameSegment || !segment.Decrypted {
				continue
			}
			frames++
			if segment.Frame.Header.Code != 91 {
				continue
			}
			r := decoder.Decode(record.Direction, segment.Frame)
			f := r.Fields
			sub, ok := f["subcommand"].(uint8)
			if !ok {
				continue
			}
			server := record.Direction == capture.ServerToClient
			if server && sub == 103 {
				active = f["map_id"] == int32(1121)
				requests, confirmed, shots, cards = nil, nil, nil, nil
				boss, damage, hp, allowance, fires = 0, 0, 0, 0, 0
			}
			if !active {
				continue
			}
			switch sub {
			case 64:
				if server && f["max_hp"] == int32(3500) {
					boss = f["living_id"].(int32)
				}
			case 32:
				if server {
					confirmed = append(confirmed, f["template_id"].(int32))
				} else {
					requests = append(requests, f["template_id"].(int32))
				}
			case 46:
				if server {
					allowance = f["shoot_count"].(int8)
				}
			case 2:
				if !server {
					shots = append(shots, []int32{f["x"].(int32), f["y"].(int32), f["force"].(int32), f["angle"].(int32)})
					continue
				}
				fires++
				for _, projectile := range f["projectiles"].([]map[string]any) {
					for _, action := range projectile["actions"].([]map[string]any) {
						if action["type"] == int32(5) && action["target_id"] == boss {
							damage += action["damage"].(int32)
							hp = action["hp"].(int32)
						}
					}
				}
			case 98, 114:
				if !server && f["card_index"] != int8(100) {
					cards = append(cards, sub)
				}
			default:
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if frames != 3372 || !reflect.DeepEqual(requests, []int32{10002, 10004, 10005, 10000}) || !reflect.DeepEqual(confirmed, []int32{10002, 10004, 10005}) || allowance != 2 || len(shots) != 2 || !reflect.DeepEqual(shots[0], shots[1]) || fires != 2 || damage != 4146 || hp != -646 || !reflect.DeepEqual(cards, []uint8{98, 98, 114}) {
		t.Fatalf("frames=%d requests=%v confirmed=%v allowance=%d shots=%d fires=%d damage=%d hp=%d card kinds=%v", frames, requests, confirmed, allowance, len(shots), fires, damage, hp, cards)
	}
	after, err := os.ReadFile(path)
	if err != nil || sha256.Sum256(after) != before {
		t.Fatal("capture changed")
	}
}
