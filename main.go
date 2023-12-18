package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"sync"
	"time"
)

type Result struct {
	Output      string `json:"output"`
	ReturnCode  int    `json:"return_code"`
	DateCommand string `json:"date_command"`
	Command     string `json:"command"`
	Hostname    string `json:"hostname"`
}

type SSHOutput struct {
	Results []Result `json:"results"`
}

func pingServer(server string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(server, "22"), 500*time.Millisecond)
	if err != nil {
		log.Printf("Server not reachable, error: %v", err)
		return false
	}
	defer conn.Close()
	return true
}

func main() {
	serversFile := flag.String("l", "", "File with a list of servers")
	user := flag.String("u", "", "User")
	command := flag.String("c", "", "Command to execute")
	flag.Parse()

	file, err := os.Open(*serversFile)
	if err != nil {
		log.Fatalf("Failed to open file: %s", err)
	}
	defer file.Close()

	var servers []string
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanWords) // Split by whitespace characters
	for scanner.Scan() {
		servers = append(servers, scanner.Text())
	}

	var wg sync.WaitGroup
	var results []Result
	var mu sync.Mutex // Mutex to protect the results slice

	sem := make(chan struct{}, 500) // Semaphore to limit concurrent SSH connections

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Println("Interrupt signal received. Gracefully shutting down...")
		close(sem) // Close the semaphore channel to release all goroutines waiting on it
		os.Exit(1)
	}()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Unable to get user home directory: %v", err)
		return
	}
	keyPath := path.Join(homeDir, ".ssh", "id_rsa")

	key, err := ioutil.ReadFile(keyPath)
	if err != nil {
		log.Printf("Unable to read private key: %v", err)
		return
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Printf("Unable to parse private key: %v", err)
		return
	}

	config := &ssh.ClientConfig{
		User: *user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	for _, server := range servers {
		wg.Add(1)
		go func(server string) {
			defer wg.Done()

			if !pingServer(server) {
				log.Printf("Server %s not reachable. Skipping.", server)
				mu.Lock()
				results = append(results, Result{
					Output:      fmt.Sprintf("Server %s not reachable. Skipping.", server),
					ReturnCode:  1,
					DateCommand: time.Now().Format(time.RFC3339),
					Command:     *command,
					Hostname:    server,
				})
				mu.Unlock()
				return
			}

			select {
			case sem <- struct{}{}: // Acquire a token
				defer func() { <-sem }() // Release the token when done
			default:
				log.Printf("Semaphore limit reached for %s. Skipping.", server)
				return
			}

			config.Timeout = 2 * time.Second
			client, err := ssh.Dial("tcp", server+":22", config)
			if err != nil {
				log.Printf("Failed to dial: %s", err)

				mu.Lock()
				results = append(results, Result{
					Output:      fmt.Sprintf("Failed to dial: %s", err),
					ReturnCode:  1,
					DateCommand: time.Now().Format(time.RFC3339),
					Command:     *command,
					Hostname:    server,
				})
				mu.Unlock()
				return
			}

			session, err := client.NewSession()
			if err != nil {
				log.Printf("Failed to create session: %s", err)
				return
			}
			defer session.Close()

			var stdoutBuf bytes.Buffer
			session.Stdout = &stdoutBuf
			err = session.Run(*command)

			var returnCode int
			if err != nil {
				if exitError, ok := err.(*ssh.ExitError); ok {
					returnCode = exitError.ExitStatus()
				} else {
					log.Printf("Failed to run: %s", err)
					return
				}
			}

			dateCommand := time.Now().Format(time.RFC3339)

			hostnameSession, err := client.NewSession()
			if err != nil {
				log.Printf("Failed to create session: %s", err)
				return
			}
			defer hostnameSession.Close()

			var hostnameBuf bytes.Buffer
			hostnameSession.Stdout = &hostnameBuf
			err = hostnameSession.Run("hostname")
			if err != nil {
				log.Printf("Failed to run: %s", err)
				return
			}

			result := Result{
				Output:      stdoutBuf.String(),
				ReturnCode:  returnCode,
				DateCommand: dateCommand,
				Command:     *command,
				Hostname:    server,
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(server)
	}

	wg.Wait()

	output := SSHOutput{
		Results: results,
	}

	if len(results) > 0 {
		log.Println("Ecriture du fichier")
		jsonOutput, err := json.Marshal(output)
		if err != nil {
			log.Fatalf("Failed to marshal output: %s", err)
		}

		date := time.Now().Format("2006-01-02")
		fileName := fmt.Sprintf("gssh-%s.log", date)
		err = ioutil.WriteFile(fileName, jsonOutput, 0644)
		if err != nil {
			log.Fatalf("Failed to write output to file: %s", err)
		} else {
			log.Printf("Output written to %s", fileName)
		}
	} else {
		log.Println("No results to marshal.")
	}
}
