package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	// "os/signal"
	"strconv"
	"strings"
	"sync"
	// "syscall"
)

type Server struct {
	Name string
	Addr string
	Port string
}

var myServer = Server{}


// var interrupted bool = false

var balance map[string]int                     // key: account, value: balance
var transactions map[string][]string           // key: txid, value: txs
var relevantAccount map[string]map[string]bool // key: txid, key: account, value: is used

var txMtx sync.Mutex
var balanceMtx sync.Mutex
var balanceCpMtx sync.Mutex
var connMtx sync.Mutex
var relevantAccountMtx sync.Mutex

// message format: addr:port:id operation
func handleConn(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		msg, err := reader.ReadString('\n')
		fmt.Fprintf(os.Stderr, "Server "+myServer.Name +" received message: "+msg)
		if err != nil {
			break
		}
		// fmt.Fprintf(os.Stderr, msg)
		msg = msg[:len(msg)-1]
		dat := strings.Split(msg, " ")
		if _, found := relevantAccount[dat[0]]; !found {
			relevantAccount[dat[0]] = make(map[string]bool)
		}
		if _, foundTransaction := transactions[dat[0]]; !foundTransaction {
			txMtx.Lock()
			transactions[dat[0]] = make([]string, 0)
			txMtx.Unlock()
		}
		switch {
		case dat[1] == "COMMIT":
			if isValid(dat[0]) {
				fmt.Fprintf(conn, dat[0]+" COMMIT OK\n")
			} else {
				rollBack(transactions[dat[0]])
				txMtx.Lock()
				transactions[dat[0]] = make([]string, 0)
				txMtx.Unlock()
				fmt.Fprintf(conn, dat[0]+" ABORTED\n")
			}
		case dat[1] == "ABORT":
			rollBack(transactions[dat[0]])
			txMtx.Lock()
			transactions[dat[0]] = make([]string, 0)
			txMtx.Unlock()
			fmt.Fprintf(conn, dat[0]+" ABORTED\n")
		case dat[1] == "BALANCE": // txID BALANCE A.foo
			fmt.Fprintf(conn, dat[0]+" "+dat[2]+" = "+strconv.Itoa(balance[dat[2]])+"\n")
		case dat[1] == "DEPOSIT": // txID DEPOSIT A.foo 10
			relevantAccountMtx.Lock()

			relevantAccount[dat[0]][dat[2]] = true
			relevantAccountMtx.Unlock()

			amount, _ := strconv.Atoi(dat[3])
			balanceMtx.Lock()
			balance[dat[2]] += amount
			balanceMtx.Unlock()

			txMtx.Lock()
			transactions[dat[0]] = append(transactions[dat[0]], msg[len(dat[0])+1:])
			n, e := fmt.Fprintf(conn, dat[0]+" OK\n")
			fmt.Println("Sent message to service", n, e)

			txMtx.Unlock()

		case dat[1] == "WITHDRAW": // txID WITHDRAW B.bar 30
			relevantAccountMtx.Lock()
			relevantAccount[dat[0]][dat[2]] = true
			relevantAccountMtx.Unlock()

			amount, _ := strconv.Atoi(dat[3])
			balanceMtx.Lock()
			balance[dat[2]] -= amount
			balanceMtx.Unlock()

			txMtx.Lock()
			transactions[dat[0]] = append(transactions[dat[0]], msg[len(dat[0])+1:])
			fmt.Fprintf(conn, dat[0]+" OK\n")
			txMtx.Unlock()
		}
	}
}

func rollBack(txList []string) {
	txListLen := len(txList)
	for i := txListLen - 1; i >= 0; i-- {
		dat := strings.Split(txList[i], " ")
		switch {
		case dat[0] == "DEPOSIT":
			amount, _ := strconv.Atoi(dat[2])
			balanceMtx.Lock()
			balance[dat[1]] -= amount
			balanceMtx.Unlock()
		case dat[0] == "WITHDRAW":
			amount, _ := strconv.Atoi(dat[2])
			balanceMtx.Lock()
			balance[dat[1]] += amount
			balanceMtx.Unlock()
		}
	}
}

func isValid(txID string) bool {
	for k, v := range relevantAccount[txID] {
		if balance[k] < 0 && v == true {
			return false
		}
	}
	return true
}

func initialize_serverInfo(name string, config string){
	myServer.Name = name
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
			myServer.Addr = str[1]
			myServer.Port = str[2]
			break
		}
	}
	fmt.Fprintf(os.Stdout, "Server %s with address %s and port %s starts", myServer.Name, myServer.Addr, myServer.Port)
}

func main() {

	// parameter
	argv := os.Args[1:]
	if len(argv) != 2 {
		fmt.Fprintf(os.Stderr, "Server argument: ./serve <NAME> <CONFIG FILE>\n")
		os.Exit(1)
	}
	initialize_serverInfo(argv[0],argv[1])
	// maps
	balance = make(map[string]int)
	transactions = make(map[string][]string)
	relevantAccount = make(map[string]map[string]bool)
	// listen
	ln, e := net.Listen("tcp", ":"+myServer.Port)
	if e != nil {
		fmt.Fprintf(os.Stderr, "Server listen error\n")
		os.Exit(1)
	}
	conn, acceptErr := ln.Accept()
	if acceptErr != nil {
		fmt.Fprintf(os.Stderr, "Server accept error\n")
		os.Exit(1)
	}
	// handleConn
	handleConn(conn)
}
