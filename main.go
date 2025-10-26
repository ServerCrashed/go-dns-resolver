package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

type cacheEntry struct {
	msg       []byte
	timestamp time.Time
	ttl       time.Duration
}

var (
	cache   = make(map[string]cacheEntry)
	cacheMu sync.RWMutex
)

func main() {
	addr := net.UDPAddr{Port: 8053, IP: net.ParseIP("0.0.0.0")}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	log.Println("DNS Resolver running on :8053")

	buf := make([]byte, 512)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Println("Read error:", err)
			continue
		}

		q := make([]byte, n)
		copy(q, buf[:n])
		go handleRequest(conn, clientAddr, q)
	}
}

func handleRequest(conn *net.UDPConn, addr *net.UDPAddr, query []byte) {
	qname, qtype := parseQuery(query)
	cacheKey := fmt.Sprintf("%s:%d", qname, qtype)

	cacheMu.RLock()
	entry, ok := cache[cacheKey]
	cacheMu.RUnlock()
	if ok {
		if time.Since(entry.timestamp) < entry.ttl {
			log.Printf("Cache hit for %s", qname)
			resp := make([]byte, len(entry.msg))
			copy(resp, entry.msg)
			if len(resp) >= 2 && len(query) >= 2 {
				copy(resp[0:2], query[0:2])
			}
			conn.WriteToUDP(resp, addr)
			return
		}
		cacheMu.Lock()
		delete(cache, cacheKey)
		cacheMu.Unlock()
	}

	resp, ttl := resolveUpstream(query)
	if resp == nil {
		log.Println("Failed to resolve", qname)
		return
	}

	respCopy := make([]byte, len(resp))
	copy(respCopy, resp)
	cacheMu.Lock()
	cache[cacheKey] = cacheEntry{msg: respCopy, timestamp: time.Now(), ttl: ttl}
	cacheMu.Unlock()
	conn.WriteToUDP(resp, addr)
}

func resolveUpstream(query []byte) ([]byte, time.Duration) {
	server := "8.8.8.8:53"
	conn, err := net.Dial("udp", server)
	if err != nil {
		log.Println("Dial error:", err)
		return nil, 0
	}
	defer conn.Close()

	_, err = conn.Write(query)
	if err != nil {
		log.Println("Write to upstream failed:", err)
		return nil, 0
	}

	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		log.Println("Read from upstream failed:", err)
		return nil, 0
	}

	resp := make([]byte, n)
	copy(resp, buf[:n])
	ttl := parseTTL(resp)
	return resp, ttl
}

func parseTTL(resp []byte) time.Duration {
	if len(resp) < 12 {
		return 0
	}
	qdCount := binary.BigEndian.Uint16(resp[4:6])
	i := 12
	for q := 0; q < int(qdCount); q++ {
		for i < len(resp) && resp[i] != 0 {
			jump := int(resp[i]) + 1
			i += jump
		}
		if i >= len(resp) {
			return 0
		}

		i++

		i += 4
	}
	if i+10 >= len(resp) {
		return 0
	}
	ttl := binary.BigEndian.Uint32(resp[i+4 : i+8])
	return time.Duration(ttl) * time.Second
}

func parseQuery(packet []byte) (string, uint16) {
	var qnameParts []string
	i := 12
	for i < len(packet) {
		length := int(packet[i])
		if length == 0 {
			i++
			break
		}
		if i+1+length > len(packet) {
			break
		}
		qnameParts = append(qnameParts, string(packet[i+1:i+1+length]))
		i += length + 1
	}
	qname := strings.Join(qnameParts, ".")
	var qtype uint16
	if i+2 <= len(packet) {
		qtype = binary.BigEndian.Uint16(packet[i : i+2])
	}
	return qname, qtype
}
