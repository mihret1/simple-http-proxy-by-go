package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"strconv"
	"sync"
	"time"
)


var backendQueue chan *Backend
var requestBytes map[string]int64
var requestLock sync.Mutex

func init() {
	requestBytes = make(map[string]int64)
	backendQueue = make(chan *Backend, 10)
}



type Backend struct {
	net.Conn
	Reader *bufio.Reader
	Writer *bufio.Writer
}

type Empty struct{}
type Status struct {
	RequestBytes map[string]int64
}



type RpcServer struct{}

func (r *RpcServer) GetStatus(args *Empty, reply *Status) error {
	requestLock.Lock()
	defer requestLock.Unlock()

	reply.RequestBytes = make(map[string]int64)
	for k, v := range requestBytes {
		reply.RequestBytes[k] = v
	}
	return nil
}


func getBackend() (*Backend, error) {
	select {
	case be := <-backendQueue:
		return be, nil
	case <-time.After(100 * time.Millisecond):
		be, err := net.Dial("tcp", "127.0.0.1:8079")
		if err != nil {
			return nil, err
		}
		return &Backend{
			Conn:   be,
			Reader: bufio.NewReader(be),
			Writer: bufio.NewWriter(be),
		}, nil
	}
}

func queueBackend(be *Backend) {
	select {
	case backendQueue <- be:
	default:
		be.Close()
	}
}



func updateStats(req *http.Request, resp *http.Response) int64 {
	requestLock.Lock()
	defer requestLock.Unlock()

	bytes := requestBytes[req.URL.Path] + resp.ContentLength
	requestBytes[req.URL.Path] = bytes

	return bytes
}


func handleConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	for {
		req, err := http.ReadRequest(reader)
		if err != nil {
			if err != io.EOF {
				log.Printf("Failed to read request: %s", err)
			}
			return
		}

		be, err := getBackend()
		if err != nil {
			log.Printf("Failed to get backend: %s", err)
			return
		}

		if err := req.Write(be.Writer); err == nil {
			be.Writer.Flush()
		}

		resp, err := http.ReadResponse(be.Reader, req)
		if err != nil {
			log.Printf("Failed to read response: %s", err)
			return
		}

		bytes := updateStats(req, resp)
		resp.Header.Set("X-Bytes", strconv.FormatInt(bytes, 10))

		if err := resp.Write(conn); err != nil {
			log.Printf("Failed to write response: %s", err)
		}

		go queueBackend(be)
	}
}



func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello from the backend server!")
	})

	log.Fatal(http.ListenAndServe(":8079", nil))
	rpc.Register(&RpcServer{})
	rpc.HandleHTTP()

	go func() {
		l, err := net.Listen("tcp", ":8079")
		if err != nil {
			log.Fatalf("Failed to listen: %s", err)
		}

		err = http.Serve(l, nil)
		if err != nil {
			log.Fatalf("Failed to serve HTTP: %s", err)
		}
	}()

	ln, err := net.Listen("tcp", ":8070")
	if err != nil {
		log.Fatalf("Failed to listen: %s", err)
	}

	for {
		conn, err := ln.Accept()
		if err == nil {
			go handleConnection(conn)
		}
	}
}