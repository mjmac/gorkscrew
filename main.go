package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"

	"golang.org/x/net/proxy"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s -t (http|socks5) <proxyhost> <proxyport> <desthost> <destport> [authfile]\n", os.Args[0])
}

func fatal(err error) {
	if err == nil {
		err = errors.New("unknown error")
	}
	fmt.Fprintf(os.Stderr, "ERROR: %s\n", err.Error())
	os.Exit(-1)
}

func httpProxy(proxyAddr, destAddr string) (net.Conn, error) {
	// dial the proxy
	conn, err := net.Dial("tcp", proxyAddr)
	if err != nil {
		return nil, err
	}

	// attempt to establish the connection
	fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", destAddr, proxyAddr)
	br := bufio.NewReader(conn)
	res, err := http.ReadResponse(br, nil)
	if err != nil {
		return nil, fmt.Errorf("reading HTTP response from CONNECT to %s via proxy %s failed: %v",
			destAddr, proxyAddr, err)
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("proxy error from %s while dialing %s: %v", proxyAddr, destAddr, err)
	}

	return conn, nil
}

func main() {
	var (
		pType = flag.String("t", "http", "proxy type")
	)
	flag.Parse()

	if flag.NArg() < 4 || flag.NArg() > 5 {
		usage()
		os.Exit(-1)
	}
	if flag.NArg() == 5 {
		fatal(errors.New("proxy authorization not supported yet"))
	}
	args := flag.Args()
	proxyAddr := net.JoinHostPort(args[0], args[1])
	destAddr := net.JoinHostPort(args[2], args[3])

	var conn net.Conn
	var err error

	switch strings.ToLower(*pType) {
	case "http":
		conn, err = httpProxy(proxyAddr, destAddr)
	case "socks5":
		dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, nil)
		if err != nil {
			fatal(err)
		}
		conn, err = dialer.Dial("tcp", destAddr)
		if err != nil {
			fatal(err)
		}
	default:
		fatal(fmt.Errorf("unsupported proxy type %q", *pType))
	}

	if err != nil {
		fatal(fmt.Errorf("failed to dial %s: %s", destAddr, err))
	}

	// define a fn to copy in to out
	xfer := func(errors chan<- error, dst io.WriteCloser, src io.ReadCloser) {
		defer dst.Close()
		defer src.Close()
		if _, err := io.Copy(dst, src); err != nil {
			errors <- err
		}
	}

	// wire up the ins/outs and fire off the copies
	errChan := make(chan error)
	go xfer(errChan, os.Stdout, conn)
	go xfer(errChan, conn, os.Stdin)

	// wait for something to fail, or ctrl-c
	err = <-errChan
	if err != nil && err != io.EOF {
		fatal(err)
	}
}
