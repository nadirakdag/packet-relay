package main

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"sync"
)

// ListenerConfig represents a single listener's configuration
type ListenerConfig struct {
	Protocol      string   `json:"protocol"`
	ListenAddr    string   `json:"listenAddr"`
	TargetServers []string `json:"targetServers"`
}

// AppConfig represents the overall application configuration
type AppConfig struct {
	Listeners []ListenerConfig `json:"listeners"`
}

func main() {
	// Load configuration
	config, err := loadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// WaitGroup to handle multiple listeners
	var wg sync.WaitGroup

	// Start listeners
	for _, listener := range config.Listeners {
		wg.Add(1)
		go func(listener ListenerConfig) {
			defer wg.Done()
			if listener.Protocol == "udp" {
				startUDPListener(listener)
			} else if listener.Protocol == "tcp" {
				startTCPListener(listener)
			} else {
				log.Printf("Unknown protocol: %s", listener.Protocol)
			}
		}(listener)
	}

	// Wait for all listeners to finish (runs indefinitely)
	wg.Wait()
}

// loadConfig reads the configuration file and parses it into AppConfig
func loadConfig(filename string) (*AppConfig, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config AppConfig
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	return &config, err
}

// startUDPListener starts a UDP listener based on the given configuration
func startUDPListener(config ListenerConfig) {
	conn, err := net.ListenPacket("udp", config.ListenAddr)
	if err != nil {
		log.Printf("Error starting UDP listener on %s: %v", config.ListenAddr, err)
		return
	}
	defer conn.Close()
	log.Printf("UDP listener started on %s", config.ListenAddr)

	buf := make([]byte, 1024)
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			log.Printf("Error reading UDP packet: %v", err)
			continue
		}
		log.Printf("Received UDP packet from %s", addr)

		for _, server := range config.TargetServers {
			go func(server string, data []byte) {
				udpAddr, err := net.ResolveUDPAddr("udp", server)
				if err != nil {
					log.Printf("Error resolving UDP server %s: %v", server, err)
					return
				}
				_, err = conn.WriteTo(data, udpAddr)
				if err != nil {
					log.Printf("Error forwarding UDP to %s: %v", server, err)
				} else {
					log.Printf("Forwarded UDP packet to %s", server)
				}
			}(server, buf[:n])
		}
	}
}

// startTCPListener starts a TCP listener based on the given configuration
func startTCPListener(config ListenerConfig) {
	listener, err := net.Listen("tcp", config.ListenAddr)
	if err != nil {
		log.Printf("Error starting TCP listener on %s: %v", config.ListenAddr, err)
		return
	}
	defer listener.Close()
	log.Printf("TCP listener started on %s", config.ListenAddr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting TCP connection: %v", err)
			continue
		}
		log.Printf("New TCP connection from %s", conn.RemoteAddr())

		go handleTCPConnection(conn, config.TargetServers)
	}
}

// handleTCPConnection handles a single TCP connection and forwards data to target servers
func handleTCPConnection(conn net.Conn, targetServers []string) {
	defer conn.Close()

	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				log.Printf("Connection closed by %s", conn.RemoteAddr())
			} else {
				log.Printf("Error reading from TCP connection: %v", err)
			}
			break
		}

		for _, server := range targetServers {
			go func(server string, data []byte) {
				targetConn, err := net.Dial("tcp", server)
				if err != nil {
					log.Printf("Error connecting to TCP server %s: %v", server, err)
					return
				}
				defer targetConn.Close()

				_, err = targetConn.Write(data)
				if err != nil {
					log.Printf("Error forwarding TCP to %s: %v", server, err)
				} else {
					log.Printf("Forwarded TCP data to %s", server)
				}
			}(server, buf[:n])
		}
	}
}
