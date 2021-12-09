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

var balance map[string]int                     // record the balance for each account
var transactions map[string][]string           // record the transactions and the operations in each transactions
var AccountInvolved map[string][]string  	   // record the account involved in a certain transactions
var AccountCreated map[string][]string		   // record the account created in a certain transactions
var TxAborted map[string]bool           	   // record wether the transactions has aborted

var TxLock sync.Mutex		//lock for transactions
var BlLock sync.Mutex		//lock for balance
var AILock sync.Mutex		//lock for AccountInvolved
var ACLock sync.Mutex		//lock for AccountInvolved
var TxALock sync.Mutex   	//lock for TxAborted

func TxValid(ID string) bool {
	for _,account := range AccountInvolved[ID] {
		if balance[account] < 0 {
			return false
		}
	}
	return true
}

// message format: addr:port:id operation
func handleConn(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		msg, err := reader.ReadString('\n')
		//fmt.Fprintf(os.Stderr, "Server "+myServer.Name +" received message: "+msg)
		if err != nil {
			//fmt.Fprintf(os.Stderr, "Server "+myServer.Name +" readstring error\n")
			os.Exit(-1)
		}
		// fmt.Fprintf(os.Stderr, msg)
		if msg[len(msg)-1] == '\n'{
			msg = msg[:len(msg)-1]
		}
		op := strings.Split(msg, " ")
		if _, AIMapExist := AccountInvolved[op[0]]; !AIMapExist {
			AILock.Lock()
			AccountInvolved[op[0]] = make([]string, 0)
			AILock.Unlock()
		}
		if _, TxExist := transactions[op[0]]; !TxExist {
			TxLock.Lock()
			transactions[op[0]] = make([]string, 0)
			TxLock.Unlock()
			TxALock.Lock()
			TxAborted[op[0]] = false
			TxALock.Unlock()
		}
		if _, ACMapExist := transactions[op[0]]; !ACMapExist {
			TxLock.Lock()
			transactions[op[0]] = make([]string, 0)
			TxLock.Unlock()
		}
		switch {
		case op[1] == "COMMIT":
			if TxValid(op[0]) {
				fmt.Fprintf(conn, op[0]+" COMMIT OK\n")
			} else {
				TxALock.Lock()
				TxAborted[op[0]] = true
				TxALock.Unlock()
				rollBack(op[0])
				TxLock.Lock()
				transactions[op[0]] = make([]string, 0)
				TxLock.Unlock()
				fmt.Fprintf(conn, op[0]+" ABORTED\n")
			}
		case op[1] == "ABORT":
			TxALock.Lock()
			if !TxAborted[op[0]]{
				TxAborted[op[0]] = true
				rollBack(op[0])
				TxLock.Lock()
				transactions[op[0]] = make([]string, 0)
				TxLock.Unlock()
				fmt.Fprintf(conn, op[0]+" ABORTED\n")
			}
			TxALock.Unlock()
		case op[1] == "BALANCE": // txID BALANCE A.foo
			BlLock.Lock()
			_, ok := balance[op[2]];
			BlLock.Unlock()
			if !ok {
				TxALock.Lock()
				TxAborted[op[0]] = true
				TxALock.Unlock()
				rollBack(op[0])
				fmt.Fprintf(conn, op[0]+" NOT FOUND, ABORTED\n")
			}else {
				BlLock.Lock()
				fmt.Fprintf(conn, op[0]+" "+op[2]+" = "+strconv.Itoa(balance[op[2]])+"\n")
				BlLock.Unlock()
			}
		case op[1] == "DEPOSIT": // txID DEPOSIT A.foo 10
			BlLock.Lock()
			_, ok := balance[op[2]];
			BlLock.Unlock()
			if !ok{
				balance[op[2]] = 0
				AccountCreated[op[0]] = append(AccountCreated[op[0]], op[2])
			}
			AILock.Lock()
			AccountInvolved[op[0]] = append(AccountInvolved[op[0]], op[2])
			AILock.Unlock()

			amount, _ := strconv.Atoi(op[3])
			BlLock.Lock()
			balance[op[2]] += amount
			BlLock.Unlock()

			TxLock.Lock()
			transactions[op[0]] = append(transactions[op[0]], msg[len(op[0])+1:])
			n, e := fmt.Fprintf(conn, op[0]+" OK\n")
			fmt.Println("Sent message to service", n, e)
			TxLock.Unlock()

		case op[1] == "WITHDRAW": // txID WITHDRAW B.bar 30
			BlLock.Lock()
			_, ok := balance[op[2]];
			BlLock.Unlock()
			if !ok {
				TxALock.Lock()
				TxAborted[op[0]] = true
				TxALock.Unlock()
				rollBack(op[0])
				fmt.Fprintf(conn, op[0]+" NOT FOUND, ABORTED\n")
			}else {
				AILock.Lock()
				AccountInvolved[op[0]] = append(AccountInvolved[op[0]], op[2])
				AILock.Unlock()
	
				amount, _ := strconv.Atoi(op[3])
				BlLock.Lock()
				balance[op[2]] -= amount
				BlLock.Unlock()
	
				TxLock.Lock()
				transactions[op[0]] = append(transactions[op[0]], msg[len(op[0])+1:])
				fmt.Fprintf(conn, op[0]+" OK\n")
				TxLock.Unlock()
			}
		}
	}
}

func rollBack(ID string) {
	List := transactions[ID]
	AccountList := AccountCreated[ID]
	length := len(List)
	AccountLen := len(AccountList)
	for i := length - 1; i >= 0; i-- {
		op := strings.Split(List[i], " ")
		switch {
		case op[0] == "DEPOSIT":
			amount, _ := strconv.Atoi(op[2])
			BlLock.Lock()
			balance[op[1]] -= amount
			BlLock.Unlock()
		case op[0] == "WITHDRAW":
			amount, _ := strconv.Atoi(op[2])
			BlLock.Lock()
			balance[op[1]] += amount
			BlLock.Unlock()
		}
	}
	for i := 0; i < AccountLen; i++ {
		_, ok := balance[AccountList[i]];
		if ok {
			delete(balance, AccountList[i]);
		}
	}
}

func initialize_serverInfo(name string, config string){
	myServer.Name = name
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
		if str[0]==name{
			myServer.Addr = str[1]
			myServer.Port = str[2]
			break
		}
	}
	//fmt.Fprintf(os.Stdout, "Server %s with address %s and port %s \n", myServer.Name, myServer.Addr, myServer.Port)
}

func main() {

	// parameter
	argv := os.Args[1:]
	if len(argv) != 2 {
		//fmt.Fprintf(os.Stderr, "Server argument: ./serve <NAME> <CONFIG FILE>\n")
		os.Exit(1)
	}
	initialize_serverInfo(argv[0],argv[1])
	// maps
	balance = make(map[string]int)
	transactions = make(map[string][]string)
	AccountInvolved  = make(map[string][]string)
	AccountCreated = make(map[string][]string)
	TxAborted = make(map[string]bool)
	// listen
	ln, e := net.Listen("tcp", ":"+myServer.Port)
	if e != nil {
		//fmt.Fprintf(os.Stderr, "Server listen error\n")
		os.Exit(1)
	}
	conn, acceptErr := ln.Accept()
	if acceptErr != nil {
		//fmt.Fprintf(os.Stderr, "Server accept error\n")
		os.Exit(1)
	}
	// handleConn
	handleConn(conn)
}

