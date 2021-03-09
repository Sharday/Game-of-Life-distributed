package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

const alive = 255
const dead = 0

const (
	quitting int = iota
	pausing
	resuming
	shuttingDown
)

var numAlive int
var turnTotal int = -1
var currWorld [][]byte
var mutex = &sync.Mutex{}
var flags = make(chan int)
var workerAddr []string
var shutDown = false
var okToClose = make(chan bool, 1)
var closeDown = make(chan bool)

func makeMatrix(height, width int) [][]byte {
	matrix := make([][]byte, height)
	for i := range matrix {
		matrix[i] = make([]byte, width)
	}
	return matrix
}

func worker(width, startY, endY, worldHeight int, workerAddr string, world [][]byte, out chan<- [][]byte, req stubs.RequestGame) {

	//part of old world to compare to
	oldHeight := endY - startY + 2
	oldWorldPart := makeMatrix(oldHeight, width)
	for y, j := startY, 1; y < endY; y, j = y+1, j+1 {
		for x := 0; x < width; x++ {
			oldWorldPart[j][x] = world[y][x]
		}
	}

	if (startY - 1) >= 0 {
		oldWorldPart[0] = world[startY-1]
	} else {
		oldWorldPart[0] = world[worldHeight-1]
	}

	if endY < worldHeight {
		oldWorldPart[oldHeight-1] = world[endY]
	} else {
		oldWorldPart[oldHeight-1] = world[0]
	}

	//connect to worker
	client, err := rpc.Dial("tcp", workerAddr)
	if err != nil {
		fmt.Println("error with connecting to worker")
		fmt.Println(err)
	}
	defer client.Close()

	//call calculatePartNextState
	request := stubs.PartNextStateRequest{Width: width, Height: worldHeight, WorldPart: oldWorldPart}
	response := new(stubs.PartNextStateResponse)
	err = client.Call(stubs.PartNextState, request, response)
	if err != nil {
		fmt.Println("error with client.Call for worker")
		fmt.Println(err)
	}
	out <- response.World

}

func calculateNextState(numWorkers int, world [][]byte, out []chan [][]byte) [][]byte {

	newWorld := makeMatrix(0, 0)

	//populate new world with new parts
	for i := 0; i < numWorkers; i++ {
		part := <-out[i]
		newWorld = append(newWorld, part...)
	}

	return newWorld
}

func calculateAliveCells(p stubs.RequestGame, world [][]byte) []util.Cell {
	aliveCells := []util.Cell{}

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == alive {
				aliveCells = append(aliveCells, util.Cell{X: x, Y: y})
			}
		}
	}

	return aliveCells
}

func closeWorkers(numWorkers int) {
	for i := 0; i < numWorkers; i++ {
		//connect to worker
		client, err := rpc.Dial("tcp", workerAddr[i])
		if err != nil {
			fmt.Println("error with connecting to worker")
			fmt.Println(err)
		}
		defer client.Close()

		//shut down worker
		request := stubs.CloseRequest{}
		response := new(stubs.CloseResponse)
		err = client.Call(stubs.CloseWorker, request, response)
		if err != nil {
			fmt.Println("error with client.Call for shutting down Worker: ")
			fmt.Println(err)
		}
	}
}

type GOL struct{}

func (g *GOL) CloseDistributor(req stubs.CloseRequest, res *stubs.CloseResponse) (err error) {
	<-okToClose
	closeDown <- true
	return
}

func (g *GOL) SignalStateChange(req stubs.StateChange, res *stubs.StateChangeResponse) (err error) {
	flags <- req.StateFlag
	res.Turn = turnTotal
	return
}

func (g *GOL) GetCurrentState(req stubs.StateRequest, res *stubs.StateResponse) (err error) {
	mutex.Lock()
	res.Turn = turnTotal
	res.World = currWorld
	mutex.Unlock()
	return
}

func (g *GOL) GetAliveCells(req stubs.AliveCellsRequest, res *stubs.AliveCellsResponse) (err error) {
	mutex.Lock()
	res.NumAlive = numAlive
	res.Turn = turnTotal
	mutex.Unlock()
	return
}

func (g *GOL) Distributor(req stubs.RequestGame, res *stubs.ResponseGame) (err error) {

	workerAddr = req.WorkerIPs
	numWorkers := len(req.WorkerIPs)

	world := req.World
	currWorld = req.World

	out := make([]chan [][]byte, numWorkers)
	for i := range out {
		out[i] = make(chan [][]byte)
	}

	//distribute workload
	workerHeight := req.ImageHeight / numWorkers
	remainder := req.ImageHeight % numWorkers
	workLoad := make([]int, numWorkers)
	for i := range workLoad {
		workLoad[i] = workerHeight
	}

	for j := 0; j < remainder; j++ {
		workLoad[j]++
	}

	exit := false
	turnTotal = req.StartingTurn
	numAlive = len(calculateAliveCells(req, world))
	for turn := req.StartingTurn; turn < req.Turns; turn++ {

		//start workers
		endY := 0
		var startY int
		for i := 0; i < numWorkers; i++ {
			startY = endY
			endY = startY + workLoad[i]
			go worker(req.ImageWidth, startY, endY, req.ImageHeight, workerAddr[i], world, out[i], req)
		}
		mutex.Lock()

		world = calculateNextState(numWorkers, world, out)
		turnTotal = turn + 1
		numAlive = len(calculateAliveCells(req, world))
		currWorld = world

		mutex.Unlock()

		select {
		case flag := <-flags:
			switch flag {
			case quitting:
				exit = true
			case pausing:
				flag = <-flags
			case shuttingDown:
				shutDown = true
				exit = true
			}
		default:
			break
		}

		if exit {
			break
		}
	}

	res.TurnsCompleted = turnTotal
	res.AliveCells = calculateAliveCells(req, world)
	res.World = world

	if shutDown {
		closeWorkers(numWorkers)
		okToClose <- true
	}

	return
}

func listenForShutDown() {
	<-closeDown
	time.Sleep(time.Millisecond * 500)
	os.Exit(0)
}

func main() {

	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()

	rpc.Register(&GOL{})
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		fmt.Println("error with net.Listen")
		fmt.Println(err)
	}

	go listenForShutDown()

	defer listener.Close()
	rpc.Accept(listener)

}
