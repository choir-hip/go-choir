//go:build linux

package main

import (
	"fmt"
	"io"
	"log"
	"net"

	"golang.org/x/sys/unix"
)

// An ecosystem_proxy (L2) capsule's network namespace has one interface,
// lo. The broker brings it up and splices each loopback TCP connection onto
// the executor's egress socket, so the only way out is the proxy outside the
// capsule (docs/design/engineering-network-grants-2026-10-10.md §4.0b). The
// forwarder is a byte pipe; every policy decision is made by the proxy.

// bringLoopbackUp sets IFF_UP on lo. It needs CAP_NET_ADMIN in the capsule's
// user namespace, so it runs before the broker drops capabilities.
func bringLoopbackUp() error {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("loopback control socket: %w", err)
	}
	defer unix.Close(fd)
	ifr, err := unix.NewIfreq("lo")
	if err != nil {
		return err
	}
	if err := unix.IoctlIfreq(fd, unix.SIOCGIFFLAGS, ifr); err != nil {
		return fmt.Errorf("read lo flags: %w", err)
	}
	ifr.SetUint16(ifr.Uint16() | unix.IFF_UP)
	if err := unix.IoctlIfreq(fd, unix.SIOCSIFFLAGS, ifr); err != nil {
		return fmt.Errorf("bring lo up: %w", err)
	}
	return nil
}

// startEgressForwarder listens on the in-capsule proxy address and forwards
// each connection to socketPath.
func startEgressForwarder(listenAddr, socketPath string) (net.Listener, error) {
	if err := bringLoopbackUp(); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("egress forwarder listen: %w", err)
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("capsule-broker: egress forwarder stopped: %v", err)
				return
			}
			go forwardEgress(conn, socketPath)
		}
	}()
	return listener, nil
}

func forwardEgress(client net.Conn, socketPath string) {
	defer client.Close()
	upstream, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Printf("capsule-broker: egress socket unavailable: %v", err)
		return
	}
	defer upstream.Close()
	done := make(chan struct{}, 2)
	pipe := func(dst, src net.Conn) {
		_, _ = io.Copy(dst, src)
		if cw, ok := dst.(interface{ CloseWrite() error }); ok {
			_ = cw.CloseWrite()
		} else {
			_ = dst.Close()
		}
		done <- struct{}{}
	}
	go pipe(upstream, client)
	go pipe(client, upstream)
	<-done
	<-done
}
