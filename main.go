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