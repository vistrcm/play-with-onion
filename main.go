package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"github.com/fatih/color"
	"github.com/go-pkgz/lgr"
	"github.com/google/uuid"
	"golang.org/x/net/proxy"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	revision   = "dev"
	nodeID     = getNodeId()
	listenAddr = flag.String("listen", ":8080", "Address to listen on")
	socksProxy = flag.String("socks-proxy", "", "Socks proxy to use")
	l          = setupLogger()
)

func setupLogger() *lgr.Logger {
	colorizer := lgr.Mapper{
		ErrorFunc:  func(s string) string { return color.New(color.FgHiRed).Sprint(s) },
		WarnFunc:   func(s string) string { return color.New(color.FgHiYellow).Sprint(s) },
		InfoFunc:   func(s string) string { return color.New(color.FgHiWhite).Sprint(s) },
		DebugFunc:  func(s string) string { return color.New(color.FgWhite).Sprint(s) },
		CallerFunc: func(s string) string { return color.New(color.FgBlue).Sprint(s) },
		TimeFunc:   func(s string) string { return color.New(color.FgCyan).Sprint(s) },
	}
	l := lgr.New(lgr.Msec, lgr.Map(colorizer))
	return l
}

type message struct {
	From    string
	Message string
}

func getNodeId() string {
	return uuid.New().String()
}

func listener(quit chan bool) {
	ln, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		l.Logf("FATAL problem listening on %q: %q\n", *listenAddr, err)
	}
	defer func(ln net.Listener) {
		err := ln.Close()
		if err != nil {
			l.Logf("FATAL problem closing listener: %q", err)
		}
	}(ln)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				l.Logf("WARN problem accepting connection: %q", err)
				continue
			}
			go handleConnection(conn)
		}
	}()

	<-quit
	err = ln.Close()
	if err != nil {
		l.Logf("FATAL problem closing listener: %q", err)
	}
}

func handleConnection(conn net.Conn) {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			l.Logf("FATAL problem closing connection: %q", err)
		}
	}(conn)
	dec := gob.NewDecoder(conn)
	enc := gob.NewEncoder(conn)
	for {
		var msg message
		err := dec.Decode(&msg)
		if err != nil {
			l.Logf("FATAL problem decoding from connection: %q", err)
		}
		l.Logf("DEBUG Request from: %q: %q\n", msg.From, msg.Message)
		resp := message{
			From:    nodeID,
			Message: fmt.Sprintf("echo from %q: %q", nodeID, msg.Message),
		}
		l.Logf("DEBUG Reply to: %q: %q\n", msg.From, resp.Message)
		err = enc.Encode(resp)
		if err != nil {
			l.Logf("FATAL problem encoding in response to connection: %q\n", err)
		}
	}
}

func main() {
	flag.Parse()
	l.Logf("INFO Starting node %q. Revision: %s", nodeID, revision)

	quitListener := make(chan bool)
	go listener(quitListener)

	peers := buildPeers()
	quitPing := make(chan bool)
	go ping(quitPing, peers, 15*time.Second)

	// handle signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigs
	l.Logf("INFO Received signal %s, shutting down", sig)
	quitPing <- true
	quitListener <- true
	l.Logf("INFO bye")
}

func buildPeers() []*Peer {
	peers := make([]*Peer, 0)
	targets := flag.Args()
	if len(targets) == 0 {
		l.Logf("FATAL no ping target specified. Please specify a targets to ping as arguments")
	}

	dialer := getDialer()
	for _, target := range targets {
		peer := NewPeer(dialer, target)
		peers = append(peers, &peer)
	}
	return peers
}

func ping(ping chan bool, peers []*Peer, period time.Duration) {
	for _, peer := range peers {
		go func(peer *Peer) {
			defer func() {
				err := peer.close()
				if err != nil {
					l.Logf("FATAL problem closing peer: %q", err)
				}
			}()

			ticker := time.NewTicker(period)
			defer ticker.Stop()
			for ; true; <-ticker.C {
				peer.ping()
			}
		}(peer)
	}
	<-ping
}

func getDialer() Dialer {
	if *socksProxy == "" {
		return &net.Dialer{}
	}

	l.Logf("INFO Using socks proxy %q", *socksProxy)
	proxyDialer, err := proxy.SOCKS5("tcp", *socksProxy, nil, proxy.Direct)
	if err != nil {
		l.Logf("FATAL problem creating proxy dialer: %q", err)
	}
	return proxyDialer
}
