package protobuf

import (
	"errors"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/sleep2death/gothic"
)

var (
	opened int
)

// Serve ...
func Serve(addr string) {
	var events gothic.Events

	events.Serving = func(s gothic.Server) (action gothic.Action) {
		log.Println("[gothic protobuf server started]")
		return
	}

	events.Opened = func(c gothic.Conn) (out []byte, action gothic.Action) {
		opened++
		c.SetContext(newContext())
		return
	}

	events.Closed = func(c gothic.Conn) (action gothic.Action) {
		opened--
		return
	}

	events.Data = func(c gothic.Conn, in []byte) (out []byte, action gothic.Action) {
		ctx := c.Context().(*context)
		data := ctx.is.Begin(in)
		// log.Printf("received %d", len(data))

		// handle the input []byte
		for {
			if len(data) >= frameHeaderLen {
				fh := &FrameHeader{
					Length: (uint32(data[0])<<16 | uint32(data[1])<<8 | uint32(data[2])),
					Type:   FrameType(data[3]),
					Flags:  Flags(data[4]),
				}

				err := fh.validate()
				if err != nil {
					// TODO: write error to out
					log.Printf("frameheader not valid: %v", err)
					return nil, gothic.Close
				}

				ctx.fh = fh
				body := data[frameHeaderLen:]
				// log.Println(fh.Length)

				if len(body) >= int(fh.Length) {
					log.Printf("expect len: %d, actual len:%d, data: %s", fh.Length, len(data), string(body[:fh.Length]))
					// ctx.is.End(data[fh.Length:])
					data = body[fh.Length:]
					if len(data) > 0 {
						log.Println("more frame(s)...")
					}
					// ctx.is.End(data)
				} else {
					// not enough data for body parsing
					log.Println("body length not enough...")
					ctx.is.End(data)
					break
				}
			} else {
				// not enough data for header parsing
				if len(data) > 0 {
					log.Println("frameheader length not enough...")
				}
				ctx.is.End(data)
				break
			}
		}
		return
	}

	log.Fatal(gothic.Serve(events, "tcp://"+addr))
}

// FRAME -------------------------------------------------

// A FrameType is a registered frame type as defined in
// http://http2.github.io/http2-spec/#rfc.section.11.2
type FrameType uint8

const frameHeaderLen = 5

const (
	// FrameData type
	FrameData FrameType = 0x0
	// FrameSettings type
	FrameSettings FrameType = 0x1
	// FramePing type
	FramePing FrameType = 0x2
)

var frameName = map[FrameType]string{
	FrameData:     "DATA",
	FrameSettings: "SETTINGS",
	FramePing:     "PING",
}

func (t FrameType) String() string {
	if s, ok := frameName[t]; ok {
		return s
	}
	return fmt.Sprintf("UNKNOWN_FRAME_TYPE_%d", uint8(t))
}

const (
	minMaxFrameSize = 1 << 14
	maxFrameSize    = 1<<24 - 1
)

// Flags is a bitmask of HTTP/2 flags.
// The meaning of flags varies depending on the frame type.
type Flags uint8

// Has reports whether f contains all (0 or more) flags in v.
func (f Flags) Has(v Flags) bool {
	return (f & v) == v
}

// Frame-specific FrameHeader flag bits.
const (
	// check flag for validating the frame
	FlagFrameAck Flags = 0x10

	// Data Frame
	// FlagDataEndStream Flags = 0x10

	// Settings Frame
	// FlagSettingsAck Flags = 0x10

	// Ping Frame
	// FlagPingAck Flags = 0x10
)

// ErrFrameTooLarge is returned from Framer.ReadFrame when the peer
// sends a frame that is larger than declared with SetMaxReadFrameSize.
var ErrFrameTooLarge = errors.New("tcp: frame too large")

// ErrFrameFlags is returned from ReadFrame when Flags.has returned false
var ErrFrameFlags = errors.New("tcp: frame flags error")

// FrameHeader store the reading data header
type FrameHeader struct {
	// Type is the 1 byte frame type.
	Type FrameType
	// Flags are the 1 byte of 8 potential bit flags per frame.
	// They are specific to the frame type.
	Flags Flags
	// Length is the length of the frame, not including the 9 byte header.
	// The maximum size is one byte less than 16MB (uint24), but only
	// frames up to 16KB are allowed without peer agreement.
	Length uint32
}

func (fh *FrameHeader) validate() error {
	// frame body size check
	if fh.Length > maxFrameSize {
		return ErrFrameTooLarge
	}

	// frameack flag check for validating the data
	if fh.Flags.Has(FlagFrameAck) == false {
		return ErrFrameFlags
	}

	// TODO: specific frame type check
	return nil
}

// ReadFrameHeader from the given io reader
func ReadFrameHeader(r io.Reader) (FrameHeader, error) {
	pbuf := fhBytes.Get().(*[]byte)
	defer fhBytes.Put(pbuf)

	buf := *(pbuf)

	_, err := io.ReadFull(r, buf[:frameHeaderLen])

	if err != nil {
		return FrameHeader{}, err
	}

	fh := FrameHeader{
		Length: (uint32(buf[0])<<16 | uint32(buf[1])<<8 | uint32(buf[2])),
		Type:   FrameType(buf[3]),
		Flags:  Flags(buf[4]),
	}

	err = fh.validate()
	return fh, err
}

// WriteData writes a data frame.
func WriteData(out []byte) (frame []byte, err error) {
	var flags Flags
	// flags |= FlagDataEndStream
	flags |= FlagFrameAck

	length := len(out)
	if length >= (1 << 24) {
		return nil, ErrFrameTooLarge
	}

	header := [frameHeaderLen]byte{
		byte(length >> 16),
		byte(length >> 8),
		byte(length),
		byte(FrameData),
		byte(flags),
	}
	frame = append(header[:frameHeaderLen], out...)
	return
}

// frame header bytes pool.
// Used only by ReadFrameHeader.
var fhBytes = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, frameHeaderLen)
		return &buf
	},
}

// InputStream is a helper type for managing input streams from inside
// the Data event.
type InputStream struct{ b []byte }

// Begin accepts a new packet and returns a working sequence of
// unprocessed bytes.
func (is *InputStream) Begin(packet []byte) (data []byte) {
	data = packet
	if len(is.b) > 0 {
		is.b = append(is.b, data...)
		data = is.b
	}
	return data
}

// End shifts the stream to match the unprocessed data.
func (is *InputStream) End(data []byte) {
	if len(data) > 0 {
		if len(data) != len(is.b) {
			is.b = append(is.b[:0], data...)
		}
	} else if len(is.b) > 0 {
		is.b = is.b[:0]
	}
}

// Context of the tcp connection
type context struct {
	is *InputStream
	fh *FrameHeader
}

var ctxPool sync.Pool

func newContext() *context {
	if v := ctxPool.Get(); v != nil {
		ctx := v.(*context)
		return ctx
	}
	return &context{is: &InputStream{}, fh: &FrameHeader{}}
}

func putContext(ctx *context) {
	// reset context
	ctx.is.b = nil

	ctx.fh.Length = 0
	ctx.fh.Flags = 0
	ctx.fh.Type = 0

	ctxPool.Put(ctx)
}
