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

	recieve "github.com/rakulmaria/handin_04/grpc"
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
		clients:          make(map[int32]recieve.RecieveClient),
		ctx:              ctx,
	}

	// Create listener tcp on port ownPort
	list, err := net.Listen("tcp", "localhost"+fmt.Sprintf(":%v", ownPort))
	if err != nil {
		log.Fatalf("Failed to listen on port: %v", err)
	}
	grpcServer := grpc.NewServer()
	recieve.RegisterRecieveServer(grpcServer, p)

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
		c := recieve.NewRecieveClient(conn)
		p.clients[port] = c
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		p.Enter()
	}
}

type peer struct {
	recieve.UnimplementedRecieveServer
	id               int32
	lamport          int32
	isUsing          bool
	amountOfRequests map[int32]int32
	deferQueue       []int32
	state            State
	clients          map[int32]recieve.RecieveClient
	ctx              context.Context
	channel          chan recieve.Request
}

func (p *peer) Recieve(ctx context.Context, req *recieve.Request) (*recieve.Reply, error) {
	id := req.Id
	p.lamport = req.Lamport;
	p.lamport++;
	fmt.Printf("peer with id: %v now has lamportclock: %v",id,p.lamport)
	
	p.amountOfRequests[id] += 1
	// check if you sent a request yourself && check if you are in the critical section.
	// in case you are requesting at the same time as the other, the one with the smallest lamport ts wins
	// otherwise, you defer the request
	if p.state == "held" || (p.state == "wanted" && (req.Lamport > p.lamport)) {
		//p.deferQueue = append(p.deferQueue, req.Id)
		fmt.Print("State held or wanted with smallest lamport")
		time.Sleep(5 * time.Second)
		reply := p.Exit()
		fmt.Println("Released")
		p.lamport++;
		fmt.Printf("peer with id: %v now has lamportclock: %v",id,p.lamport)
		return reply, nil
	} else {
		fmt.Printf("you take it person with id: %v", id)
		p.lamport++;
		fmt.Printf("peer with id: %v now has lamportclock: %v",id,p.lamport)
		rep := &recieve.Reply{Id: p.amountOfRequests[id]}
		return rep, nil
	}
}

func (p *peer) Enter() {
	isAvailable := false
	p.state = "wanted"
	p.lamport++;
	fmt.Printf("peer with id: %v now has lamportclock: %v",p.id,p.lamport)
	request := &recieve.Request{Id: p.id, Lamport: p.lamport}
	for _, client := range p.clients {
		rep, err := client.Recieve(p.ctx, request)
		p.lamport = rep.Lamport
		p.lamport++;
		fmt.Printf("peer with id: %v now has lamportclock: %v",p.id,p.lamport)
		if err != nil {
			fmt.Println("something went wrong")
		}
	}
	fmt.Printf("recieved message from everone. Person with id: %v now has the thing", p.id)
	isAvailable = true
	//recieved all replies
	if(isAvailable){
		p.state = "held"
	}
}

func (p *peer) Exit()(*recieve.Reply){
	fmt.Printf("peer with id: %v exited the thing", p.id)
	p.state = "realeased"
	rep := &recieve.Reply{Id: p.id, Lamport: p.lamport}
	return rep
}


// our enum for State
type State string

const (
	released       = "released"
	held           = "held"
	wanted         = "wanted"
	undefinedState = "illegal"
)
