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
	"time"
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
	currentTx	string // timestamp 
	conn		net.Conn
	readLock	sync.Mutex // the lock to read what Client send
}

type Transaction struct{
	id			string // "client.id-client.currentTx"
	locks		[]*Account
	mtx			sync.Mutex // lock to communicate with Client
	commitCount	int
	conn		net.Conn
	aborted     bool
	commited	bool
	ServerInvolved map[string]*Server
}

type Account struct{
	name		string // in the format of "A.foo"
	RWLock		sync.RWMutex // locking call: readLock.RLock()/RUnlock()
	status 		int 	// 0 for no lock, 1 for read lock, 2 for write lock
	statusLock 	sync.Mutex
	owner		string // the id of client
}

var branchTable map[string]*Server
var clientTable map[string]*Client
var AccountTable map[string]*Account
var transactionTable map[string]*Transaction
var txTable sync.Mutex

var ATLock sync.Mutex

var myServer = Server{}

func genTimestamp() string{
	return fmt.Sprintf("%f", float64(time.Now().UnixNano())/float64(time.Second))
}

func initialize_serverInfo(name string, config string){
	myServer.id = name
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
			myServer.hostname = str[1]
			myServer.port = str[2]
		}else{
			branchTable[str[0]] = &Server{
				id: str[0],
				hostname: str[1],
				port: str[2],
			}
		}
	}
}

// all possible reply from BRANCH
// txID COMMIT OK
// txID ABORTED
// txID OK
func handleRep(rep string, branch *Server){
	//fmt.Fprintf(os.Stderr,"rep: "+rep)
	if rep[len(rep)-1] == '\n'{
		rep = rep[:len(rep)-1]
	}
	frags := strings.SplitN(rep," ",2)
	txID := frags[0]
	transaction, found := transactionTable[txID]
	if found{
		transaction.mtx.Lock()
		chars := strings.Split(transaction.id,"-")
		client := clientTable[chars[0]]
		if frags[1] == "COMMIT OK"{
			transaction.commitCount--
			if transaction.commitCount==0{
				fmt.Fprintf(transaction.conn, frags[1]+"\n")
				client.readLock.Unlock()
				for _, account := range transaction.locks{
					account.statusLock.Lock()
					if account.status == 1{
						account.RWLock.RUnlock()
					}else if account.status == 2{
						account.RWLock.Unlock()
					}
					account.owner = ""
					account.status = 0
					account.statusLock.Unlock()
				}
				transaction.commited = true

			}
		}else if frags[1] == "ABORTED"{
				abort(transaction, frags[1]+"\n")
		}else if frags[1] == "NOT FOUND, ABORTED"{
				abort(transaction, frags[1]+"\n")
		}else{
			// the only possible replies
			// A.foo = 10 or OK
			// redirect to client directly
			fmt.Fprintf(transaction.conn, frags[1]+"\n")
			client.readLock.Unlock()
		}
		transaction.mtx.Unlock()
	}else{
		//fmt.Fprintf(os.Stderr, "handleRep: can't find current transaction in txTable!\n")
	}
}

func handleBranch(branch *Server){
	reader := bufio.NewReader(branch.conn)
	for {
		rep, err := reader.ReadString('\n')
		if err != nil{
			//fmt.Fprintf(os.Stderr, "Branch is disconnected from Server\n")
			break
		}
		go handleRep(rep,branch)
	}
}

// the full name of each transaction
func (client *Client) formTxID() string{
	return client.id + "-" + client.currentTx
}

func addNewTx(txID string, CONN net.Conn){
	transactionTable[txID] = &Transaction{
		id:				txID,
		locks:			make([]*Account,0),
		mtx:			sync.Mutex{}, // multi branch/client communicate with the same transaction
		commitCount: 	0,
		conn:			CONN,
		ServerInvolved: make(map[string]*Server),
		aborted:		false,
		commited:		false,
	}
}

func abort(tx *Transaction, printStr string){
	if tx.aborted == true{
		return
	}
	for _, account := range tx.locks{
		account.statusLock.Lock()
		if account.status == 1{
			account.RWLock.RUnlock()
		}else if account.status == 2{
			account.RWLock.Unlock()
		}
		account.status = 0
		account.owner = ""
		account.statusLock.Unlock()
		op := strings.Split(account.name,".")
		branch := branchTable[op[0]]
		fmt.Fprintf(branch.conn, tx.id + " ABORT\n")
	}
	tx.aborted = true
	chars := strings.Split(tx.id,"-")
	client := clientTable[chars[0]]
	//fmt.Fprintf(os.Stderr,printStr)
	fmt.Fprintf(tx.conn,printStr)
	client.readLock.Unlock()
}

func commit(tx *Transaction){
	tx.mtx.Lock()
	//fmt.Fprintf(os.Stderr,"enter commit function\n")
	for _,account := range tx.locks{
		op := strings.Split(account.name,".")
		branch := branchTable[op[0]]
		//fmt.Fprintf(os.Stderr,"send msg to "+op[0]+"\n")
		fmt.Fprintf(branch.conn, tx.id + " COMMIT\n")
	}
	tx.mtx.Unlock()
}

func addAcc2Tx(tx *Transaction, ac *Account){
	for _, account := range tx.locks{
		if account.name == ac.name{
			return
		}
	}
	tx.locks = append(tx.locks,ac)
}


func write(tx *Transaction, account string, value int, cmd string){
	// txid DEPOSIT A.foo 10
	// txid WITHDRAW B.bar 30
	tx.mtx.Lock()

	ATLock.Lock()

	_, ok := AccountTable[account]
	if !ok {
		AccountTable[account] = &Account{
			name:		account,
			RWLock:		sync.RWMutex{},
			status:		0,
			statusLock: sync.Mutex{},
			owner:		"",
		}
	}
	tmpAccount := AccountTable[account]
	ATLock.Unlock()
	addAcc2Tx(tx,tmpAccount)
	op := strings.Split(cmd, " ")
	branchName := strings.Split(op[1], ".")
	branch := branchTable[branchName[0]]
	_, ok2 := tx.ServerInvolved[branchName[0]]
	if !ok2 {
		tx.ServerInvolved[branchName[0]]=branch
		tx.commitCount++
	}
	chars := strings.Split(tx.id,"-")
	tmpAccount.statusLock.Lock()
	if tmpAccount.status != 2{
		tmpAccount.status = 2
		tmpAccount.owner = chars[0]
		tmpAccount.RWLock.Lock()
		tmpAccount.statusLock.Unlock()
	}else{
		tmpAccount.statusLock.Unlock()
		if tmpAccount.owner != chars[0]{
			tmpAccount.RWLock.Lock()
		}
		tmpAccount.statusLock.Lock()
		tmpAccount.status = 2
		tmpAccount.owner = chars[0]
		tmpAccount.statusLock.Unlock()
	}

	fmt.Fprintf(branch.conn, tx.id + " %s\n", cmd)
	//fmt.Fprintf(os.Stderr, "sending: "+ tx.id + " %s\n", cmd)
	tx.mtx.Unlock()
}

func read(tx *Transaction, account string, cmd string){
	// BALANCE A.foo	
	tx.mtx.Lock()
	
	ATLock.Lock()
	//fmt.Fprintf(os.Stderr, "enter read: "+cmd)
	_, ok := AccountTable[account]
	if !ok {
		AccountTable[account] = &Account{
			name:		account,
			RWLock:		sync.RWMutex{},
			status:		0,
			statusLock: sync.Mutex{},
			owner:		"",
		}
	}
	tmpAccount := AccountTable[account]
	ATLock.Unlock()
	addAcc2Tx(tx,tmpAccount)
	op := strings.Split(cmd, " ")
	branchName := strings.Split(op[1], ".")
	branch := branchTable[branchName[0]]
	_, ok2 := tx.ServerInvolved[branchName[0]]
	if !ok2 {
		tx.ServerInvolved[branchName[0]]=branch
		tx.commitCount++
	}

	chars := strings.Split(tx.id,"-")
	tmpAccount.statusLock.Lock()
	//fmt.Fprintf(os.Stderr,"tmpAccount.status = "+strconv.Itoa(tmpAccount.status))
	if tmpAccount.status == 0{
		tmpAccount.status = 1
		tmpAccount.RWLock.RLock()
		//fmt.Fprintf(os.Stderr,"here?")
		tmpAccount.statusLock.Unlock()
	}else if tmpAccount.status == 1{
		tmpAccount.RWLock.RLock()
		tmpAccount.statusLock.Unlock()
	}else{
		//status == 2
		if tmpAccount.owner != chars[0]{
			tmpAccount.statusLock.Unlock()
			tmpAccount.RWLock.RLock()
			tmpAccount.statusLock.Lock()
			tmpAccount.status = 1
			tmpAccount.statusLock.Unlock()
		}else{
			tmpAccount.statusLock.Unlock()
		}
	}

	fmt.Fprintf(branch.conn, tx.id + " %s\n", cmd)
	
	tx.mtx.Unlock()
}


func handleClient(client *Client){
	reader := bufio.NewReader(client.conn)
	for {
		cmd, err := reader.ReadString('\n')
		if err != nil{
			//fmt.Fprintf(os.Stderr,"can't read Client!\n")
			os.Exit(1)
		}
		handleCMD(cmd,client)
	}
}
// all the possible CMDs from clients:
// BEGIN
// COMMIT
// DEPOSIT A.foo 10
// WITHDRAW B.bar 30
// BALANCE A.foo
// ABORT
func handleCMD(cmd string, client *Client){
	if cmd == "\n" {
		return
	}
	//fmt.Fprintf(os.Stderr, "cmd: "+ cmd)
	client.readLock.Lock() 
	if cmd[len(cmd)-1] == '\n'{
		cmd = cmd[:len(cmd)-1]
	}
	if cmd == "BEGIN"{
		client.currentTx = genTimestamp()
		txTable.Lock()
		addNewTx(client.formTxID(),client.conn)
		fmt.Fprintf(client.conn,"OK\n")
		client.readLock.Unlock()
		txTable.Unlock()
	}else{
		transaction,found := transactionTable[client.formTxID()]
		if found{
			if transaction.aborted == true || transaction.commited == true{
				client.readLock.Unlock()
				return
			}
			if cmd == "COMMIT"{
				commit(transaction)
			}else if cmd == "ABORT"{
				transaction.mtx.Lock()
				abort(transaction,"ABORTED\n")
				transaction.mtx.Unlock()
			}else{
				// at this point all the CMD are related to read&write
				frags := strings.Split(cmd," ")
				if len(frags) == 3{
					// DEPOSIT A.foo 10
					// WITHDRAW B.bar 30 
					val, err := strconv.Atoi(frags[2])
					if err != nil{
						//fmt.Fprintf(os.Stderr, "can't translate string to int!\n")
					}
					if frags[0] == "WITHDRAW"{
						val = -val
					}
					write(transaction, frags[1], val, cmd)
				}else{
					// BALANCE A.foo
					read(transaction,frags[1],cmd)
				}
			}
		}else{
			//fmt.Fprintf(os.Stderr, "handleClient: can't find current transaction in txTable!\n")
		}
	}
}


func main(){
	// parameter
	argv := os.Args[1:]
	if len(argv) != 2 {
		fmt.Fprintf(os.Stderr, "Server argument: ./coordinator coordinator <CONFIG FILE>\n")
		os.Exit(1)
	}
	// connect to branches

	branchTable = make(map[string]*Server)
	clientTable = make(map[string]*Client)
	AccountTable = make(map[string]*Account)
	transactionTable = make(map[string]*Transaction)
	initialize_serverInfo(argv[0],argv[1])
	for _, branchServer := range branchTable{
		Conn , err := net.Dial("tcp", branchServer.hostname+":"+branchServer.port)
		if err != nil{
			//fmt.Fprintf(os.Stderr, "Error connecting to branch\n")
			os.Exit(1)
		}
		branchServer.conn = Conn
		go handleBranch(branchServer)
	}
	//fmt.Fprintf(os.Stderr, "Coordinator successfully connected to all branches\n")

	listener, err := net.Listen("tcp", ":"+myServer.port)
	if err != nil{
		//fmt.Fprintf(os.Stderr, "Error listening\n")
		os.Exit(1)
	}
	//fmt.Fprintf(os.Stderr, "Coordinator successfully listened on  its port\n")

	for{
		Conn, err := listener.Accept()
		if err != nil{
			//fmt.Fprintf(os.Stderr, "Error accepting\n")
			os.Exit(1)
		}
		clientId := Conn.RemoteAddr().String()
		clientTable[clientId] = &Client{
			id:			clientId,
			conn:		Conn,
			currentTx:	genTimestamp(),
			readLock:	sync.Mutex{},
		}
		go handleClient(clientTable[clientId]) // one handleClient thread correspond to one process
	}

}