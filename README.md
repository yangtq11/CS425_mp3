# CS425_mp3_final

## Build

```bash
sh build.sh
```

## Run

We will first run server in five different processes:

```bash
./server [ServerName] config.txt
```

Then we will run coordinator:

```bash
./coordinator coordinator config.txt
```

Finally, we will run client:

```bash
./client [ClientName] config.txt
```

## Walk-through

Suppose we have three clients in the system, all of them start simultaneously as below:

```bash
% ./client a config.txt
BEGIN
```

```bash
% ./client b config.txt
BEGIN
```

```
% ./client c config.txt
BEGIN
```

They will all be connected to the coordinator. After receiving those "`BEGIN`"s, coordinator will create three transactions on its side and reply "`OK`" simultaneously:

```bash
% ./client a config.txt
BEGIN
OK
```

```bash
% ./client b config.txt
BEGIN
OK
```

```
% ./client c config.txt
BEGIN
OK
```

Then suppose all clients send deposit to the same account name `A.foo`:

```bash
% ./client a config.txt
BEGIN
OK
DEPOSIT A.foo 10
```

```bash
% ./client b config.txt
BEGIN
OK
DEPOSIT A.foo 10
```

```
% ./client c config.txt
BEGIN
OK
DEPOSIT A.foo 10
```

After coordinator receives "`DEPOSIT`" message, it will acquire the write lock for the corresponding account. The only one thread that eventually acquires the writelock will redirect the `DEPOSIT` message to the corresponding branch server `A`. Server `A` will reply with "`OK`" and coordinator will redirect it to the corresponding client. At this point, the other two clients have to wait for the write lock to release again.

```bash
% ./client a config.txt
BEGIN
OK
DEPOSIT A.foo 10
OK
```

```bash
% ./client b config.txt
BEGIN
OK
DEPOSIT A.foo 10
```

```
% ./client c config.txt
BEGIN
OK
DEPOSIT A.foo 10
```

After a while, let's say the client a would like to commit,

```bash
% ./client a config.txt
BEGIN
OK
DEPOSIT A.foo 10
OK
COMMIT
```

```bash
% ./client b config.txt
BEGIN
OK
DEPOSIT A.foo 10
```

```
% ./client c config.txt
BEGIN
OK
DEPOSIT A.foo 10
```

The coordinator will then redirect this `COMMIT` message to all the branch servers involved in this transaction. In this case, only branch `A` will be notified. Branch `A` will then send a `COMMIT OK` to coordinator and commit the changes locally. Once coordinator has collected enough `COMMIT OK`, it will send `COMMIT OK` to the client and release all the locks acquired in this transaction. Then the thread working on another transaction (say client b) will immediately acquire the write lock of `A.foo` and send `DEPOSIT` command to branch B and send back `OK` to client 2.

```bash
% ./client a config.txt
BEGIN
OK
DEPOSIT A.foo 10
OK
COMMIT
COMMIT OK
```

```bash
% ./client b config.txt
BEGIN
OK
DEPOSIT A.foo 10
OK
BALANCE A.foo
```

```bash
% ./client c config.txt
BEGIN
OK
DEPOSIT A.foo 10
```

Now client b woud like to read the balance. The coordinator will first check the read lock, which is acquired by client b itself. After that it will redirect exactly the same message to branch B. After receiving the `BALANCE` enquiry, the branch will return the amount `20`. Coordinator will redirect exactly the same message back to client.

```bash
% ./client a config.txt
BEGIN
OK
DEPOSIT A.foo 10
OK
COMMIT
COMMIT OK
```

```bash
% ./client b config.txt
BEGIN
OK
DEPOSIT A.foo 10
OK
BALANCE A.foo
A.foo = 20
```

```bash
% ./client c config.txt
BEGIN
OK
DEPOSIT A.foo 10
```

## Concurrency

Use a two-step lockout for each account. The lock is released when the transaction is aborted or committed (using a two-phase commit). And the lock is acquired when: No other transaction has written to the account, or all transactions taking over the account are simply reading from the account.

##  Abort and Roll back
When client sends a "ABORT" to the coordinator, the coordinator server will sends an "ABORT" message to all branches. When a branch receives an "ABORT" message, it will roll back, more specifically, the branch will add back the amount of money being withdrawed or reduce the amount of money being deposited. If a branch detect some error and send abort to the coordinator, the coordinator will also send "ABORT" to all branch. Notice we have a flag called TxAborted for each transaction to mark whether the transaction has aborted. In this case, a transaction will only be aborted at most once. After the coordinator sends the abort, it will release all the locks it acquired during the transaction. Also, if an account is created in an aborted transaction, then after the abort operation, the account would not exist in the branch.
