package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	// "os/signal"
	//"strconv"
	"strings"
	//"sync"
	//"time"
	// "syscall"
)

type Server struct{
	id			string
	hostname 	string
	port		string
	conn		net.Conn
}

type Client struct{
	id			string
	conn		net.Conn
	readLock	sync.Mutex
	writeLock 	sync.Mutex
	cmdList		list.List
}

var branchTable map[string]Server
var clientTable map[string]Client
var myServer = Server{}

func initialize_serverInfo(name string, config string){
	myServer.id = name
	file, err := os.Open(config)
	if err != nil{
		fmt.Fprintf(os.Stderr, "can't open file!")
		os.Exit(-1)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	//node information
	for scanner.Scan() {
		str := strings.Split(scanner.Text(), " ")
		if str[0]==name{
			myServer.hostname = str[1]
			myServer.port = str[2]
		}else{
			branchTable[str[0]] = Server{
				id: str[0],
				hostname: str[1],
				port: str[2],
			}
		}
	}
}

func handleBranch(branchName string){

}

func handleClient(clientName string){

}

func main(){
	// parameter
	argv := os.Args[1:]
	if len(argv) != 2 {
		fmt.Fprintf(os.Stderr, "Server argument: ./coordinator coordinator <CONFIG FILE>\n")
		os.Exit(1)
	}
	// connect to branches

	branchTable = make(map[string]Server)
	initialize_serverInfo(argv[0],argv[1])
	for branchName,branchServer := range branchTable{
		Conn, err := net.Dial("tcp", branchServer.hostname+":"+branchServer.port)
		if err != nil{
			fmt.Fprintf(os.Stderr, "Error connecting to branch\n")
			os.Exit(1)
		}
		branchServer.conn = Conn
		go handleBranch(branchName)
	}

	listener, err := net.Listen("tcp", ":"+myServer.port)
	if err != nil{
		fmt.Fprintf(os.Stderr, "Error listening\n")
		os.Exit(1)
	}
	for{
		Conn, err := listener.Accept()
		if err != nil{
			fmt.Fprintf(os.Stderr, "Error accepting\n")
			os.Exit(1)
		}
		clientId := Conn.RemoteAddr().String()
		clientTable[clientId] = Client{
			id:			clientId,
			conn:		Conn,
			readLock:	sync.Mutex{}
			writeLock: 	sync.Mutex{}
			cmdList:	{}
		}
		go handleClient(clientId)
	}
}