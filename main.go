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

	enter "github.com/rakulmaria/handin_04/grpc"
	"google.golang.org/grpc"
)

// our critical section is a counter. starts at 0
var counter = 0

func main() {
	arg1, _ := strconv.ParseInt(os.Args[1], 10, 32)
	ownPort := int32(arg1) + 5000

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := &peer{
		id:               ownPort,
		amountOfRequests: make(map[int32]int32),
		clients:          make(map[int32]enter.EnterClient),
		ctx:              ctx,
	}

	// Create listener tcp on port ownPort
	list, err := net.Listen("tcp", "localhost"+fmt.Sprintf(":%v", ownPort))
	if err != nil {
		log.Fatalf("Failed to listen on port: %v", err)
	}
	grpcServer := grpc.NewServer()
	enter.RegisterEnterServer(grpcServer, p)

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
		fmt.Printf("Trying to dial: %v\n", port)
		conn, err := grpc.Dial(fmt.Sprintf(":%v", port), grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("Could not connect: %s", err)
		}
		defer conn.Close()
		c := enter.NewEnterClient(conn)
		p.clients[port] = c
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		p.SendRequestToAll()
	}
}

type peer struct {
	enter.UnimplementedEnterServer
	id               int32
	lamport          int32
	isUsing          bool
	amountOfRequests map[int32]int32
	deferQueue       []int32
	state            State
	clients          map[int32]enter.EnterClient
	ctx              context.Context
	channel          chan enter.Request
}

func Count() {
	time.Sleep(5 * time.Second)
	counter++
}

func (p *peer) Enter(ctx context.Context, req *enter.Request) (*enter.Reply, error) {
	id := req.Id
	p.amountOfRequests[id] += 1

	// check if you sent a request yourself && check if you are in the critical section.
	// in case you are requesting at the same time as the other, the one with the smallest lamport ts wins
	// otherwise, you defer the request

	if p.state == "held" || (p.state == "wanted" && (req.Lamport > p.lamport)) {
		p.deferQueue = append(p.deferQueue, req.Id)
	}

	rep := &enter.Reply{Amount: p.amountOfRequests[id]}
	return rep, nil

}

func (p *peer) SendRequestToAll() {
	request := &enter.Request{Id: p.id, Lamport: p.lamport}
	for id, client := range p.clients {
		reply, err := client.Enter(p.ctx, request)
		if err != nil {
			fmt.Println("something went wrong")
		}
		fmt.Printf("Got reply from id %v: %v\n", id, reply.Amount)
		go Count()

	}
}

// our enum for State
type State string

const (
	released       = "released"
	held           = "held"
	wanted         = "wanted"
	undefinedState = "illegal"
)
