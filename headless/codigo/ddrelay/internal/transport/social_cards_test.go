package transport

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/relay/internal/framing"
	"ddbot/relay/internal/link"
	"ddbot/relay/internal/packets"
)

func TestSocialCardsThroughRemoteBot(t *testing.T) {
	if testing.Short() {
		t.Skip("requires bot process")
	}

	cc, ctx := startBot(t, "''", func(ctx context.Context, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", address)
	})
	client, proxyClient := net.Pipe()
	server, proxyServer := net.Pipe()
	defer client.Close()
	defer server.Close()
	connection := NewConnection(1, proxyClient, proxyServer, &memoryRecorder{}, nil, packets.NewDecoder(), func(checkpoint func(context.Context) (link.Checkpoint, error)) Automation {
		return link.New(pb.NewBotClient(cc), "synthetic-test-api-key", &pb.OpenSession{
			ApiVersion:      contract.APIVersion,
			CodecRevision:   contract.CodecRevision,
			RelayInstanceId: "social-relay",
			GameSessionId:   "social-game",
			LocalStream:     1,
		}, "", slog.New(slog.NewJSONHandler(io.Discard, nil)), checkpoint)
	})
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- connection.Run(runCtx) }()
	defer func() {
		stop()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()

	for _, socket := range []net.Conn{client, server} {
		if err := socket.SetDeadline(time.Now().Add(20 * time.Second)); err != nil {
			t.Fatal(err)
		}
	}

	send := func(from, to net.Conn, data []byte) {
		t.Helper()
		written := make(chan error, 1)
		go func() { _, err := from.Write(data); written <- err }()
		got := make([]byte, len(data))
		if _, err := io.ReadFull(to, got); err != nil {
			t.Fatal(err)
		}

		if err := <-written; err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(got, data) {
			t.Fatal("forwarding changed bytes")
		}
	}
	clientKey := [8]byte{3, 19, 47, 61, 89, 103, 131, 157}
	first := encrypted(t, &clientKey, framing.Frame{Header: framing.Header{Code: 4}})
	second := encrypted(t, &clientKey, framing.Frame{Header: framing.Header{Code: 6}})
	send(client, server, append(first, second...))
	upstream := framing.NewEncryptedFramer()
	if len(upstream.Push(append(first, second...))) != 2 {
		t.Fatal("synthetic cipher not ready")
	}

	serverKey := [8]byte{11, 23, 31, 43, 59, 71, 83, 97}
	login := encrypted(t, &serverKey, framing.Frame{Header: framing.Header{Code: 1, ClientID: 123}, Body: loginBody()})
	ping := encrypted(t, &serverKey, framing.Frame{Header: framing.Header{Code: 4}, Body: wireInts(2)})
	send(server, client, append(login, ping...))
	evaluate := func(source string) *pb.ScriptResult {
		t.Helper()
		result, err := connection.Request(ctx, time.Now().Format("150405.000000000"), "eval", source)
		if err != nil {
			t.Fatal(err)
		}

		return result
	}
	wait := func(source string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)

		for {
			result, requestErr := connection.Request(ctx, time.Now().Format("150405.000000000"), "eval", source)
			if requestErr == nil && result.Error == nil {
				v, err := contract.DecodeValue(result.Value)
				if err != nil {
					t.Fatal(err)
				}

				if v == true {
					return
				}
			}

			if time.Now().After(deadline) {
				t.Fatalf("state condition failed: %s", source)
			}

			time.Sleep(time.Millisecond)
		}
	}
	wait(`return player.state().id==123`)
	response := func(code int16, body []byte) {
		t.Helper()
		send(server, client, encrypted(t, &serverKey, framing.Frame{Header: framing.Header{Code: code, ClientID: 123}, Body: body}))
	}
	receive := func(code int16, body []byte) {
		t.Helper()

		data := make([]byte, 20+len(body))
		if _, err := io.ReadFull(server, data); err != nil {
			t.Fatal(err)
		}

		segments := upstream.Push(data)
		if len(segments) != 1 || segments[0].Frame.Header.Code != code || !bytes.Equal(segments[0].Frame.Body, body) {
			t.Fatalf("unexpected command frame: %+v", segments)
		}
	}
	command := func(source string, code int16, body []byte) {
		t.Helper()
		result := evaluate(source)
		if result.Error != nil {
			t.Fatalf("command %s: %v", source, result.Error)
		}

		receive(code, body)
	}
	command(`return friends.add({name="Pal",relation=0,flag_1=false,flag_2=false})`, 160, wireJoin([]byte{160}, wireText("Pal"), wireInts(0), []byte{0, 0}))
	response(160, []byte{160, 0})
	wait(`return friends.state().last_result and friends.state().last_result.success==false`)
	friend := wireJoin([]byte{160, 1}, wireInts(42), wireText("Pal"), []byte{0}, wireInts(0), []byte{1}, wireText(""), wireText(""), wireText(""), wireInts(1, 20, 0), wireText(""), wireInts(0, 0, 0, 0, 0, 0), wireText("private"), wireInts(0, 0, 0, 0, 0), wireText(""), wireInts(0), wireText(""), wireInts(0), wireText(""), []byte{0})
	response(160, friend)
	wait(`return #friends.state().players==1 and friends.state().players[1].id==42 and friends.state().last_result.success`)
	response(94, wireJoin([]byte{0}, wireInts(9), []byte{4, 0, 1, 1, 0, 4, 0}, wireInts(2), []byte{0}, wireText("Room"), []byte{3}, wireInts(0), []byte{0, 0}))
	wait(`return room.state().id==9`)
	command(`return room.invite({player_id=42})`, 70, wireInts(42))
	response(70, wireJoin(wireInts(42, 10, 2), []byte{4, 3, 0, 0}, wireText("Pal"), []byte{0}, wireInts(0), wireText("Room"), wireText(""), wireInts(0)))
	wait(`return #room.invitations()==1 and room.invitations()[1].room_id==10`)
	command(`local i=room.invitations()[1];return room.join({login_flag=true,room_type=4,room_id=-1,argument=i.room_id,password=""})`, 94, wireJoin(wireInts(1), []byte{1}, wireInts(4, -1, 10), wireText("")))
	response(94, []byte{1, 0})
	wait(`return room.state().last_join_success==false`)
	response(94, []byte{1, 1})
	wait(`return room.state().last_join_success==true`)
	response(91, cardRoundBody(2))
	wait(`return fight.cards().known and fight.cards().type==2`)
	command(`return fight.take_card({index=0})`, 91, []byte{98, 0})
	wait(`return #fight.cards().requested==1`)
	rejected := evaluate(`return fight.take_card({index=0})`)
	if rejected.Error == nil {
		t.Fatal("duplicate selection accepted")
	}

	response(91, wireJoin([]byte{98, 0, 0}, wireInts(100, 2), []byte{0}))
	wait(`return #fight.cards().awards==1 and fight.cards().awards[1].template_id==100 and #fight.cards().requested==0`)
	command(`return fight.take_extra_card({index=1})`, 91, []byte{114, 1})
	response(3, wireJoin(wireInts(1), wireText("Extra card requires Prime")))
	wait(`return session.info().last_message and session.info().last_message.message=="Extra card requires Prime" and #fight.cards().awards==1`)
	command(`return fight.finish_cards()`, 91, []byte{98, 100})
	response(91, wireJoin([]byte{89}, wireInts(1), []byte{1}, wireInts(200, 1)))
	wait(`return fight.cards().closed and #fight.cards().revealed==1 and #fight.cards().awards==1`)
	response(91, cardRoundBody(1))
	wait(`return fight.cards().type==1 and #fight.cards().awards==0 and #fight.cards().requested==0`)
	command(`return fight.take_boss_card({index=8})`, 91, []byte{130, 8})
	response(91, wireJoin([]byte{98, 1, 8}, wireInts(300, 1)))
	wait(`return #fight.cards().awards==1 and fight.cards().awards[1].vip==nil`)
	command(`return fight.finish_boss_cards()`, 91, []byte{130, 100})
	response(91, buffFightCreateBody())
	response(91, wireJoin([]byte{103}, wireInts(30, 1120, 0, 0)))
	response(91, wireJoin([]byte{99}, wireInts(1, 0, 680, 504, -1, 1000, 1000, 1, 0, 0, 200, 0, 0), []byte{0, 0, 0, 0}, wireInts(0), []byte{7, 234, 10, 23, 12, 0, 0}))
	response(91, wireJoin([]byte{6, 1, 0, 0, 0, 0}, wireInts(30, 0, 1, 0), []byte{1}, wireInts(680, 504, 0, 1000, 1000), []byte{0}, wireInts(240, 200, 0, 0, 1, 0, 1)))
	wait(`return fight.self() and fight.self().energy==240 and fight.self().controls.known and fight.state().turn.actor_id==0`)
	command(`return fight.pow()`, 91, []byte{15, 0})
	if result := evaluate(`return fight.self().special==nil`); result.Error != nil {
		t.Fatal(result.Error)
	} else if v, err := contract.DecodeValue(result.Value); err != nil || v != true {
		t.Fatal("write alone was treated as POW acceptance")
	}

	selected := encrypted(t, &serverKey, framing.Frame{Header: framing.Header{Code: 91}, Body: wireJoin([]byte{20, 1}, wireInts(21))})
	rage := encrypted(t, &serverKey, framing.Frame{Header: framing.Header{Code: 91}, Body: wireJoin([]byte{14}, wireInts(0))})
	send(server, client, append(selected, rage...))
	wait(`return fight.self().special==true and fight.self().ball_id==21 and fight.self().dander==0`)
	command(`return fight.use_prop({bag_type=1,place=-1,template_id=10004})`, 91, wireJoin([]byte{32, 1}, wireInts(-1, 10004)))
	wait(`return fight.self().energy==nil`)
	response(91, wireJoin([]byte{32, 1}, wireInts(-1, 10004, 0), []byte{0}))
	response(91, wireJoin([]byte{41}, wireText("energy2"), wireText("155.5")))
	wait(`return fight.self().energy==155.5`)
	if result := evaluate(`fight.face({direction=-1}); fight.shoot_tag({started=true,shot_type=2}); return fight.shoot({x=680,y=504,force=670,angle=-135})`); result.Error != nil {
		t.Fatal(result.Error)
	}
	receive(91, wireJoin([]byte{7}, wireInts(-1)))
	receive(91, []byte{96, 1, 2})
	receive(91, wireJoin([]byte{2}, wireInts(680, 504, 670, -135)))
	send(server, client, encrypted(t, &serverKey, framing.Frame{Header: framing.Header{Code: 91, Parameter1: 1}, Body: wireJoin([]byte{6, 1, 0, 0, 0, 0}, wireInts(30, 0, 1, 1), []byte{1}, wireInts(253, 507, 0, 500, 500), []byte{0}, wireInts(240, 0, 0, 0, 1, 0, 2))}))
	wait(`return fight.state().turn.number==2`)
	for _, source := range []string{`return fight.pow()`, `return fight.use_prop({bag_type=1,place=-1,template_id=10004})`} {
		if result := evaluate(source); result.Error == nil {
			t.Fatal("foreign turn buff accepted")
		}
	}
	last := encrypted(t, &clientKey, framing.Frame{Header: framing.Header{Code: 4}, Body: wireInts(2)})
	written := make(chan error, 1)
	go func() { _, err := client.Write(last); written <- err }()
	translated := make([]byte, len(last))
	if _, err := io.ReadFull(server, translated); err != nil {
		t.Fatal(err)
	}

	if err := <-written; err != nil {
		t.Fatal(err)
	}

	segments := upstream.Push(translated)
	if len(segments) != 1 || segments[0].Frame.Header.Code != 4 || !bytes.Equal(segments[0].Frame.Body, wireInts(2)) {
		t.Fatal("client cipher continuity lost")
	}
}

func wireInts(values ...int32) []byte {
	var body []byte

	for _, v := range values {
		body = binary.BigEndian.AppendUint32(body, uint32(v))
	}

	return body
}

func wireText(value string) []byte {
	return append(binary.BigEndian.AppendUint16(nil, uint16(len(value))), []byte(value)...)
}

func wireJoin(values ...[]byte) []byte {
	var body []byte

	for _, v := range values {
		body = append(body, v...)
	}

	return body
}

func cardRoundBody(kind int32) []byte {
	return wireJoin([]byte{112}, wireInts(kind), []byte{0, 0}, wireInts(1, 123, 20, 1, 10), []byte{1}, wireInts(2, 0), []byte{0, 1})
}
