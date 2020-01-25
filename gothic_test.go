package gothic

import (
	"log"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGothic(t *testing.T) {
	addr := ":9001"
	var opened int
	var events Events
	events.Serving = func(s Server) (action Action) {
		go func() {
			c, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			var data [64]byte
			n, _ := c.Read(data[:])
			if string(data[:n]) != "HI THERE" {
				t.Fatalf("expected '%s', got '%s'", " THERE", data[:n])
			}
			c.Write([]byte("HELLO"))
			n, _ = c.Read(data[:])
			if string(data[:n]) != "HELLO" {
				t.Fatalf("expected '%s', got '%s'", "HELLO", data[:n])
			}
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				c, err := net.Dial("tcp", addr)
				if err != nil {
					log.Fatal(err)
				}
				defer c.Close()
				var data [64]byte
				n, _ := c.Read(data[:])
				if string(data[:n]) != "HI THERE" {
					t.Fatalf("expected '%s', got '%s'", " THERE", data[:n])
				}
				n, _ = c.Read(data[:])
				if string(data[:n]) != "HERE" {
					t.Fatalf("expected '%s', got '%s'", "HERE", data[:n])
				}
				n, _ = c.Read(data[:])
				if n != 0 {
					t.Fatalf("expected zero")
				}
				// add 15 connections
				for i := 0; i < 15; i++ {
					net.Dial("tcp", addr)
				}

			}()
			wg.Wait()

			c.Write([]byte("SHUTDOWN"))
			n, _ = c.Read(data[:])
			if string(data[:n]) != "GOOD BYE" {
				t.Fatalf("expected '%s', got '%s'", "GOOD BYE", data[:n])
			}
		}()
		return
	}
	preWriteCount := 0
	events.PreWrite = func() {
		preWriteCount++
	}
	var c2 Conn
	var max int
	events.Opened = func(c Conn) (out []byte, action Action) {
		if c.LocalAddr().String() == "" {
			t.Fatal("should not be empty")
		}
		if c.RemoteAddr().String() == "" {
			t.Fatal("should not be empty")
		}
		max++
		opened++
		if opened == 2 {
			c2 = c
		}
		return []byte("HI THERE"), None
	}
	events.Closed = func(c Conn) (action Action) {
		opened--
		return
	}
	events.Data = func(c Conn, in []byte) (out []byte, action Action) {
		if string(in) == "SHUTDOWN" {
			return []byte("GOOD BYE"), Shutdown
		}
		return in, None
	}
	numTicks := 0
	var c2n int
	events.Tick = func(now time.Time) (delay time.Duration, action Action) {
		numTicks++
		if numTicks == 1 {
			return -10, None
		}
		delay = time.Second / 10
		if c2 != nil {
			if c2n == 0 {
				c2.Write([]byte("HERE"))
			} else if c2n == 1 {
				c2.Close()
				c2 = nil
			}
			c2n++
		}
		return
	}
	if err := Serve(events, "tcp://"+addr); err != nil {
		t.Fatal(err)
	}
	if preWriteCount == 0 {
		t.Fatal("expected preWrites")
	}
	if opened != 0 {
		t.Fatal("expected zero")
	}
	if max != 17 {
		t.Fatalf("expected 17, got %v", max)
	}
	// should not cause problems
	c2.Write(nil)
	c2.Close()
}

func TestGothic2(t *testing.T) {
	addr := ":9002"

	var opened int = 0
	var count int = 50
	var events Events

	// var wg sync.WaitGroup

	var conns []net.Conn

	dial := func(count int) {
		for i := 0; i < count; i++ {
			c, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatal(err)
			}
			conns = append(conns, c)
			time.Sleep(time.Millisecond)
		}
	}

	events.Serving = func(s Server) (action Action) {
		log.Println("[start serving]")
		// go dial(count)
		return
	}

	events.Opened = func(c Conn) (out []byte, action Action) {
		if c.LocalAddr().String() == "" {
			t.Fatal("should not be empty")
		}
		if c.RemoteAddr().String() == "" {
			t.Fatal("should not be empty")
		}
		opened++
		return
	}

	events.Data = func(c Conn, in []byte) (out []byte, action Action) {
		// log.Printf("in: %s", string(in))
		if strings.Contains(string(in), "hello") {
			out = []byte("there")
			action = Close
		} else if strings.Contains(string(in), "shutdown") {
			out = []byte("bye")
			action = Shutdown
		} else {
			t.Fatal("not valid")
		}
		return
	}

	events.Closed = func(c Conn) (action Action) {
		opened--
		return
	}

	go func() {
		if err := Serve(events, "tcp://"+addr); err != nil {
			t.Fatal(err)
		}

		if opened != 0 {
			t.Fatal("all connection should closed")
		}
	}()

	time.Sleep(time.Millisecond)
	dial(count)

	for i := 0; i < len(conns); i++ {
		time.Sleep(time.Millisecond * 5)
		c := conns[i]
		c.Write([]byte("hello"))

		var data [16]byte
		n, err := c.Read(data[:])

		if err != nil {
			t.Fatal(err)
		}

		if string(data[:n]) != "there" {
			t.Fatalf("invalid response: %s", string(data[:n]))
		}
		c.Close()
		// log.Println(string(data[:n]))
	}
}
