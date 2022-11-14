package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	receive "github.com/rakulmaria/handin_04/grpc"
	"google.golang.org/grpc"
)

func main() {
	//setting the log file
	f, err := os.OpenFile("log.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)

	// the port that the peers want to join on
	arg1, _ := strconv.ParseInt(os.Args[1], 10, 32)
	ownPort := int32(arg1) + 5000

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := &peer{
		id:               ownPort,
		amountOfRequests: make(map[int32]int32),
		clients:          make(map[int32]receive.ReceiveClient),
		ctx:              ctx,
	}

	// Create listener tcp on port ownPort
	list, err := net.Listen("tcp", "localhost"+fmt.Sprintf(":%v", ownPort))
	if err != nil {
		log.Fatalf("Failed to listen on port: %v", err)
	}
	grpcServer := grpc.NewServer()
	receive.RegisterReceiveServer(grpcServer, p)

	go func() {
		if err := grpcServer.Serve(list); err != nil {
			log.Fatalf("failed to server %v", err)
		}
	}()

	for i := 0; i < 3; i++ {
		port := int32(5000) + int32(i)

		if port == ownPort {
			continue
		}

		var conn *grpc.ClientConn
		log.Printf("Trying to dial: %v\n", port)
		fmt.Printf("Trying to dial: %v\n", port)
		conn, err := grpc.Dial(fmt.Sprintf(":%v", port), grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("Could not connect: %s", err)
		}
		defer conn.Close()
		c := receive.NewReceiveClient(conn)
		p.clients[port] = c
	}

	fmt.Println("Waiting for a peer to enter the critical section ... Press enter to enter it")

	// hitting "enter" on the keyboard makes the peer request for entering the critical section
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		p.Enter()
	}
}

type peer struct {
	receive.UnimplementedReceiveServer
	id               int32
	lamport          int32
	amountOfRequests map[int32]int32
	state            State
	clients          map[int32]receive.ReceiveClient
	ctx              context.Context
	deferQueue       []int32
}

// critical section represented by a boolean
var criticalSection = false

func (p *peer) Receive(ctx context.Context, req *receive.Request) (*receive.Reply, error) {
	// if the peers lamport is less than the requested lamport, the lamport clock is updated before incremented
	if p.lamport < req.Lamport {
		p.lamport = req.Lamport
	}
	p.lamport++

	// if the peers state is held or if the state is wanted and the request's lamport clock is greater than the peers, the peer has to wait until the critical section becomes available to enter
	if p.state == held || (p.state == wanted && (req.Lamport > p.lamport)) {
		for criticalSection {
			time.Sleep(2 * time.Second)
		}
		p.lamport++
		rep := &receive.Reply{Id: p.id, Lamport: p.lamport}
		return rep, nil
	} else {
		p.lamport++
		rep := &receive.Reply{Id: p.id, Lamport: p.lamport}
		return rep, nil
	}
}

// function called when a peer wants to enter critical section
func (p *peer) Enter() {
	// the peers state is set to wanted
	p.state = wanted

	// the peer sends out a request to all the other peers for entering the critical secion
	for _, client := range p.clients {
		p.lamport++
		log.Printf("Peer with id: %v requested access to the critical section with lamporttime: %v", p.id, p.lamport)
		request := &receive.Request{Id: p.id, Lamport: p.lamport}
		rep, _ := client.Receive(p.ctx, request)
		// if the reply's lamport time is greater than the peers lamport time, the peers lamport time is updated, before incremented
		if rep.Lamport > p.lamport {
			p.lamport = rep.Lamport
		}
		p.lamport++
		log.Printf("Peer with id: %v received reply to access with lamporttime: %v", p.id, p.lamport)

	}
	//recieved all replies, updates it's state to held and enters the criticalsection represented as a boolean. stays there for five seconds
	p.state = held
	criticalSection = true
	p.lamport++
	fmt.Printf("**** Peer with id: %v entered the critical section with lamporttime: %v **** \n", p.id, p.lamport)
	log.Printf("**** Peer with id: %v entered the critical section with lamporttime: %v ****", p.id, p.lamport)
	time.Sleep(5 * time.Second)
	p.Exit()
}

// on exit, the peer changes it's state to released and leaves the criticalsection by setting it to false. the section is now up for grabs for another peer
func (p *peer) Exit() {
	p.lamport++
	p.state = released
	criticalSection = false
	fmt.Printf("---- Peer with id: %v exited the critical section with lamporttime: %v ---- \n", p.id, p.lamport)
	log.Printf("---- Peer with id: %v exited the critical section with lamporttime: %v ----", p.id, p.lamport)
}

// our enum for State
type State string

const (
	released       = "released"
	held           = "held"
	wanted         = "wanted"
	undefinedState = "illegal"
)
