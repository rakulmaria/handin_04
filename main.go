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

func main() {

	//setting the log file
	f, err := os.OpenFile("log.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)
	
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

	// Skakl nok fixes til at flere end tre kan være med eller?
	for i := 0; i < 3; i++ {
		port := int32(5000) + int32(i)

		if port == ownPort {
			continue
		}

		var conn *grpc.ClientConn
		fmt.Printf("Trying to dial: %v\n", port)
		log.Printf("Trying to dial: %v\n", port)
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
	amountOfRequests map[int32]int32
	state            State
	clients          map[int32]recieve.RecieveClient
	ctx              context.Context
	deferQueue		 []int32

}

func (p *peer) Recieve(ctx context.Context, req *recieve.Request) (*recieve.Reply, error) {
	id := req.Id
	if(p.lamport < req.Lamport) { p.lamport = req.Lamport; }
	p.lamport++;
	fmt.Println("In Receive-method")
	fmt.Printf("peer with id: %v now has lamportclock: %v\n",id,p.lamport)
	log.Printf("peer with id: %v now has lamportclock: %v and state: %v\n",id,p.lamport, p.state)
	
	p.amountOfRequests[id] += 1
	// check if you sent a request yourself && check if you are in the critical section.
	// in case you are requesting at the same time as the other, the one with the smallest lamport ts wins
	// otherwise, you defer the request
	if p.state == held || (p.state == wanted && (req.Lamport > p.lamport)) {
		p.deferQueue = append(p.deferQueue, req.Id)
		log.Printf("Person with id: %v and state: %v is now in queue\n", p.id, p.state)

		for peerId := range p.deferQueue  {
			fmt.Printf("printing all in queue: %v",peerId)
		}
		// fmt.Print("State held or wanted with smallest lamport\n")
		// time.Sleep(1 * time.Second)
		// reply := p.Exit()
		// fmt.Println("Released")
		// fmt.Println("In Receive-method - if")
		// fmt.Printf("peer with id: %v now has lamportclock: %v\n",id,p.lamport)
		//return reply, nil
		rep := &recieve.Reply{Id: 0, Lamport: 0};
		return rep, nil;
	} else {
		// fmt.Printf("you take it person with id: %v", id)
		// fmt.Println("In Receive-method - else")
		// fmt.Printf("peer with id: %v now has lamportclock: %v\n",id,p.lamport)
		// p.lamport++;
		// rep := &recieve.Reply{Id: p.amountOfRequests[id]}
		// return rep, nil
		rep,_ := p.clients[req.Id].Reply(ctx,&recieve.Empty{});
		//call reply method

		return rep, nil;
	}
}

func (p *peer) Reply(ctx context.Context, in *recieve.Empty) (*recieve.Reply, error) {
	rep := &recieve.Reply{Id: p.id, Lamport: p.lamport};
	return rep, nil;
}

func (p *peer) Enter() {
	p.state = wanted
	fmt.Printf("peer with id: %v now has lamportclock: %v\n",p.id,p.lamport)
	log.Printf("peer with id: %v now has lamportclock: %v\n",p.id,p.lamport)
	p.lamport++;
	request := &recieve.Request{Id: p.id, Lamport: p.lamport}
	for _, client := range p.clients {
		reply := &recieve.Reply{};
	
		reply,_= client.Recieve(p.ctx, request)
		if(reply.Id == 0 && reply.Lamport == 0){
			time.Sleep(10*time.Second);
			reply,_= client.Recieve(p.ctx, request)
		}
		
		if(p.lamport < reply.Lamport) { p.lamport = reply.Lamport; }
		p.lamport++;
		fmt.Printf("peer with id: %v now has lamportclock: %v\n",p.id,p.lamport)
		log.Printf("peer with id: %v now has lamportclock: %v\n",p.id,p.lamport)
	
	}
	fmt.Printf("recieved message from everone. Person with id: %v now has the thing\n", p.id)
	log.Printf("recieved message from everone. Person with id: %v now has the thing\n", p.id)
	//recieved all replies
	p.state = held
	time.Sleep(5 * time.Second)
	p.Exit();
}

func (p *peer) Exit(){
	//for all in queue call their reply
	p.state = released
	for peerId := range p.deferQueue  {
		fmt.Printf("order of queue id: %v",peerId);
		log.Printf("order of queue id: %v",peerId);
		var client = p.clients[int32(peerId)];
		if(client != nil){
		p.clients[int32(peerId)].Reply(p.ctx,&recieve.Empty{});
		} else {
			fmt.Printf("client not found from queue")
		}
		p.deferQueue = p.deferQueue[0:]
	}

	fmt.Println("In Exit-method")
	fmt.Printf("peer with id: %v exited the thing\n", p.id)
	log.Printf("peer with id: %v and state: %v exited the thing\n", p.id, p.state)
	
	p.lamport++;
	//rep := &recieve.Reply{Id: p.id, Lamport: p.lamport}
}



// our enum for State
type State string

const (
	released       = "released"
	held           = "held"
	wanted         = "wanted"
	undefinedState = "illegal"
)
