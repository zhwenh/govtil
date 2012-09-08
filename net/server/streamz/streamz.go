package streamz

import (
	"bufio"
	"bytes"
	"errors"
	"net"
	"time"
	
	"github.com/vsekhar/govtil/log"
)

var ErrOverflow = errors.New("Streamz buffer overflow")

const Timeout = 30 * time.Second

func Ticker(pub chan []byte) {
	t := time.Tick(2 * time.Second)
	var tocker bool = false
	for {
		<-t
		log.Debug("tick")
		if tocker {
			Write(pub, "tock", "tick")
		} else {
			Write(pub, "tick", "tock")
		}
		tocker = !tocker
	}
}

func DispatchForever(subs chan net.Conn, pub chan []byte) {
	sublist := make([]net.Conn, 0)
	for {
		select {
			case sub := <-subs:
				sublist = append(sublist, sub)
			case data := <-pub:
				retch := make(chan net.Conn)
				subcount := len(sublist)
				for _, sub := range sublist {
					go func(sub net.Conn) {
						sub.SetDeadline(time.Now().Add(Timeout))
						_, err := sub.Write(data)
						if err != nil {
							log.Debug("streamz timeout writing to", sub.RemoteAddr().String(), ":", err)
							retch <- nil
						} else {
							retch <- sub
						}
					}(sub)
				}

				sublist = sublist[:0]
				for i := 0; i < subcount; i++ {
					sub := <-retch
					if sub != nil {
						sublist = append(sublist, <-retch)
					}
				}
		}
	}
}

func Write(ch chan []byte, k, v string) error {
	data := []byte(k + "=" + v + "\n")
	if bytes.Count(data, []byte("=")) > 1 || bytes.Count(data, []byte("=")) > 1 {
		return errors.New("Cannot have '=' in stream key or value")
	}
	if bytes.Count(data, []byte("\n")) > 1 || bytes.Count(data, []byte("\n")) > 1 {
		return errors.New("Cannot have newline in stream key or value")
	}
	select {
	case ch <- data:
	default:
		return ErrOverflow
	}
	return nil
}

type StreamzSample struct {
	Key string
	Value string
}

// Parse a client connection to a streamz server and push StreamzSample's onto
// provided channel. On error, will close the channel.
func Channelify(conn net.Conn) chan StreamzSample {
	ch := make(chan StreamzSample)
	bio := bufio.NewReader(conn)
	go func() {
		defer func() { close(ch) }()
		for {
			line, isprefix, err := bio.ReadLine()
			if err != nil {
				log.Println("streamz: error reading line")
				return
			}
			if isprefix {
				log.Println("streamz: line too long, dying")
				return
			}
			parts := bytes.Split(line, []byte("="))
			if len(parts) != 2 {
				log.Println("streamz: malformed entry:", line)
				return
			}
			ch <- StreamzSample{Key: string(parts[0]), Value: string(parts[1])}
		}
	}()
	return ch
}
