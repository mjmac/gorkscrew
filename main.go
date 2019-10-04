//
// (C) Copyright 2019 Michael MacDonald (github@macdonald.cx)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <proxyhost> <proxyport> <desthost> <destport> [authfile]\n", os.Args[0])
}

func fatal(err error) {
	if err == nil {
		err = errors.New("unknown error")
	}
	fmt.Fprintf(os.Stderr, "ERROR: %s\n", err.Error())
	os.Exit(-1)
}

func main() {
	if len(os.Args) < 5 || len(os.Args) > 6 {
		usage()
		os.Exit(-1)
	}
	if len(os.Args) == 6 {
		fatal(errors.New("proxy authorization not supported yet"))
	}
	proxyAddr := net.JoinHostPort(os.Args[1], os.Args[2])
	destAddr := net.JoinHostPort(os.Args[3], os.Args[4])

	// dial the proxy
	conn, err := net.Dial("tcp", proxyAddr)
	if err != nil {
		fatal(fmt.Errorf("dialing proxy %q failed: %v", proxyAddr, err))
	}

	// attempt to establish the connection
	fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", destAddr, proxyAddr)
	br := bufio.NewReader(conn)
	res, err := http.ReadResponse(br, nil)
	if err != nil {
		fatal(fmt.Errorf("reading HTTP response from CONNECT to %s via proxy %s failed: %v",
			destAddr, proxyAddr, err))
	}
	if res.StatusCode != 200 {
		fatal(fmt.Errorf("proxy error from %s while dialing %s: %v", proxyAddr, destAddr, err))
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
