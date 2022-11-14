# Instructions for running program

To run the program start three terminals and run the command

    go run . X

where X is three different integers (ex. 0, 1, 2) - one for each terminal.

Once all the peers has dialed a request can be send by hitting 'ENTER' on the keyboard.

The terminal only prints when a client has successfully entered the critical section by lamport time X, and again when the clients exits the section. In the log you will find a more detailed log for every request and respond made by the peer. We decided to make the prints to the terminal minimal and instead refer to the log, because we think it looks messy and unmanageable with two different prints for every execution.
