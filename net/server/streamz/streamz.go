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

func Start(subs chan net.Conn, pub chan []byte) {
	go func() {
		sublist := make([]net.Conn, 0)
		marks := make([]int, 0)
		for {
			select {
				case sub := <-subs:
					sublist = append(sublist, sub)
				case data := <-pub:
					for i, sub := range sublist {
						_, err := sub.Write(data)
						if err != nil {
							marks = append(marks, i)
						}
					}
					
					// sweep (from the back)
					l := len(sublist)
					for len(marks) > 0 {
						i := marks[len(marks)-1]
						sublist[i].Close()
						sublist[i] = sublist[l-1]
						sublist = sublist[:l-1]
						marks = marks[:len(marks)-1]
						l--
					}
			}
		}
	}()
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

func Channelify(conn net.Conn) chan StreamzSample {
	ch := make(chan StreamzSample)
	bio := bufio.NewReader(conn)
	go func() {
		defer func() { close(ch) }()
		for {
			line, isprefix, err := bio.ReadLine()
			if err != nil {
				return
			}
			if isprefix {
				log.Fatal("Streamz line too long, dying")
			}
			parts := bytes.Split(line, []byte("="))
			if len(parts) != 2 {
				log.Fatal("Malformed streamz entry:", line)
			}
			ch <- StreamzSample{Key: string(parts[0]), Value: string(parts[1])}
		}
	}()
	return ch
}
