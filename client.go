package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	// "os/signal"
	// "syscall"
)

// const defaultServerAddr string = "172.22.156.112:4999" // VM 1

var ClientName string
var COAddr string
var COPort string

// var interrupted bool = false

func readFeedbackService(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		feedback, readErr := reader.ReadString('\n')
		if readErr != nil {
			//fmt.Fprintf(os.Stderr, "Client: Error reading feedback from server\n")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stdout, feedback)
	}
}

func initialize_ClientInfo(name string, config string){
	ClientName = name
	file, err := os.Open(config)
	if err != nil{
		//fmt.Fprintf(os.Stderr, "can't open file!")
		os.Exit(-1)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	//node information
	for scanner.Scan() {
		str := strings.Split(scanner.Text(), " ")
		if str[0]=="coordinator"{
			COAddr = str[1]
			COPort = str[2]
			break
		}
	}
}

func main() {
	argv := os.Args[1:]
	if len(argv) != 2 {
		//fmt.Fprintf(os.Stderr, "client argument: ./serve <NAME> <CONFIG FILE>\n")
		os.Exit(1)
	}
	initialize_ClientInfo(argv[0],argv[1])
	reader := bufio.NewReader(os.Stdin)

	conn, dialErr := net.Dial("tcp", COAddr+":"+COPort)
	if dialErr != nil {
		//fmt.Fprintf(os.Stderr, "Client connection error\n")
		os.Exit(1)
	}

	go readFeedbackService(conn)

	for {
		cmd, _ := reader.ReadString('\n')
		_, writeErr := fmt.Fprintf(conn, "%s\n", cmd)
		if writeErr != nil {
			//fmt.Fprintf(os.Stderr, "Client: Error writing to server\n")
			os.Exit(1)
		}
	}
}
