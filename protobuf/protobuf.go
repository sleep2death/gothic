package protobuf

import (
	"github.com/golang/protobuf/proto"
	"github.com/sleep2death/gothic"
	"log"
)

var opened int
var pdata func(c gothic.Conn, in []byte) (out []byte, err error)

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
		ctx := c.Context().(*Context)
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
					// log.Printf("expect len: %d, actual len:%d, data: %s", fh.Length, len(data), string(body[:fh.Length]))
					// TODO: any message pooling?
					var msg AnyMsg
					err := proto.Unmarshal(body[:fh.Length], &msg)

					if err != nil {
						// TODO: write error to out
						return nil, gothic.Close
					}

					ctx.ReqMsg = msg.GetAny()
					ServeProto(ctx)

					data = body[fh.Length:]
					if len(data) > 0 {
						// for multiple frames to parse
						log.Println("more frame(s)...")
					}
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

// ServeProto of the incoming message
// a shortcut for unit testing
func ServeProto(c *Context) {
}
