package protobuf

import "testing"

import "time"
import "net"

import "github.com/golang/protobuf/proto"
import "github.com/golang/protobuf/ptypes/any"

func TestServe(t *testing.T) {
	go Serve(":9001")
	time.Sleep(time.Millisecond * 5)

	conn, err := net.Dial("tcp", ":9001")
	if err != nil {
		t.Fatal(err)
	}

	// // send two frames together
	// aframe, _ := WriteData([]byte("hello"))
	// bframe, _ := WriteData([]byte("hello again"))

	// conn.Write(append(aframe, bframe...))

	// time.Sleep(time.Millisecond * 5)

	// // send a frame with unfinished header
	// packet := []byte("broken header")
	// length := len(packet)

	// half := []byte{
	// 	byte(length >> 16),
	// 	byte(length >> 8),
	// 	byte(length),
	// }
	// conn.Write(half)
	// time.Sleep(time.Millisecond * 5)

	// var flags Flags
	// flags |= FlagFrameAck
	// half = []byte{
	// 	byte(FrameData),
	// 	byte(flags),
	// }

	// half = append(half, packet...)
	// conn.Write(half)
	// time.Sleep(time.Millisecond * 5)

	// // send a frame with unfinished body
	// packet = []byte("broken body")
	// length = len(packet)

	// flags |= FlagFrameAck

	// header := []byte{
	// 	byte(length >> 16),
	// 	byte(length >> 8),
	// 	byte(length),
	// 	byte(FrameData),
	// 	byte(flags),
	// }

	// conn.Write(append(header, packet[:5]...))
	// time.Sleep(time.Millisecond * 5)

	// conn.Write(packet[5:])
	// time.Sleep(time.Millisecond * 5)

	// cframe, _ := WriteData([]byte("bye"))
	// conn.Write(cframe)
	// time.Sleep(time.Millisecond * 5)

	// protobuf
	echo := &EchoMsg{
		Message: "hello",
	}
	msg, err := proto.Marshal(echo)
	if err != nil {
		t.Fatal(err)
	}

	any := &AnyMsg{
		Any: &any.Any{
			TypeUrl: proto.MessageName(echo),
			Value:   msg,
		},
	}

	payload, err := proto.Marshal(any)
	if err != nil {
		t.Fatal(err)
	}
	pframe, _ := WriteData(payload)
	conn.Write(pframe)
	time.Sleep(time.Millisecond * 5)
}
