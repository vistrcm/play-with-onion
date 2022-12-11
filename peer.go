package main

import (
	"encoding/gob"
	"fmt"
	"net"
	"time"
)

type Dialer interface {
	Dial(network, address string) (net.Conn, error)
}

type Peer struct {
	address string
	dialer  Dialer
	conn    net.Conn
	seq     int64
	enc     *gob.Encoder
	dec     *gob.Decoder
}

// NewPeer creates a new peer struct
func NewPeer(dialer Dialer, address string) Peer {
	return Peer{
		address: address,
		dialer:  dialer,
		conn:    nil,
	}
}

// inc increment number of messages (seq) sent
func (p *Peer) inc() {
	p.seq++
}

// Connect connects to a peer
func (p *Peer) connect() error {
	conn, err := p.dialer.Dial("tcp", p.address)
	if err != nil {
		return fmt.Errorf("problem dialing %q: %w", p.address, err)
	}
	p.conn = conn
	return nil
}

// close closes the connection to a peer
func (p *Peer) close() error {
	l.Logf("INFO closing connection to %q", p.address)
	if p.conn == nil {
		return nil
	}
	err := p.conn.Close()
	if err != nil {
		return fmt.Errorf("problem closing connection: %w", err)
	}
	return nil
}

// ping sends a ping to a peer
func (p *Peer) ping() {
	p.ensureConnection()

	msg := message{
		From:    nodeID,
		Message: fmt.Sprintf("ping from %s. Seq=%d", nodeID, p.seq),
	}

	p.inc()
	start := time.Now()

	err := p.enc.Encode(msg)
	if err != nil {
		l.Logf("WARN problem encoding message to connection: %q", err)
		return
	}

	var resp message

	err = p.dec.Decode(&resp)

	if err != nil {
		l.Logf("WARN problem decoding reply: %q", err)
		return
	}
	elapsed := time.Since(start)

	l.Logf("INFO from %s: seq=%d time=%d ms\n", p.address, p.seq, elapsed.Milliseconds())
}

func (p *Peer) ensureConnection() {
	if p.conn == nil {
		for {
			l.Logf("DEBUG connecting to %q", p.address)
			err := p.connect()
			if err != nil {
				l.Logf("WARN problem connecting to %q: %q", p.address, err)
				time.Sleep(sleepDuration())
				continue
			}
			enc := gob.NewEncoder(p.conn)
			dec := gob.NewDecoder(p.conn)
			p.enc = enc
			p.dec = dec
			break
		}
	}
}

func sleepDuration() time.Duration {
	return 3 * time.Second
}
