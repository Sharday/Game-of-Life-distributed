package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
)

const alive = 255
const dead = 0

var closeDown = make(chan bool)

func mod(x, m int) int {
	return (x + m) % m
}

func makeMatrix(height, width int) [][]byte {
	matrix := make([][]byte, height)
	for i := range matrix {
		matrix[i] = make([]byte, width)
	}
	return matrix
}

func calculateNeighbours(width, height, x, y int, world [][]byte) int {
	neighbours := 0
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			if i != 0 || j != 0 {
				if world[mod(y+i, height)][mod(x+j, width)] == alive {
					neighbours++
				}
			}
		}
	}
	return neighbours
}

func calculatePartNextState(width, height int, oldWorldPart [][]byte) [][]byte {

	//make empty part
	oldHeight := len(oldWorldPart)
	newWorldPart := makeMatrix(oldHeight-2, width)

	//inspect current world part, populate new part
	for y, j := 1, 0; y < oldHeight-1; y, j = y+1, j+1 {
		for x := 0; x < width; x++ {
			neighbours := calculateNeighbours(width, height, x, y, oldWorldPart)
			if oldWorldPart[y][x] == alive {
				if neighbours == 2 || neighbours == 3 {
					newWorldPart[j][x] = alive
				} else {
					newWorldPart[j][x] = dead
				}
			} else {
				if neighbours == 3 {
					newWorldPart[j][x] = alive
				} else {
					newWorldPart[j][x] = dead
				}
			}
		}
	}
	return newWorldPart
}

func closeWorker() {
	<-closeDown
	time.Sleep(time.Millisecond * 500)
	os.Exit(0)
}

type Worker struct{}

func (w *Worker) PartNextState(req stubs.PartNextStateRequest, res *stubs.PartNextStateResponse) (err error) {
	res.World = calculatePartNextState(req.Width, req.Height, req.WorldPart)
	return
}

func (w *Worker) CloseWorker(req stubs.CloseRequest, res *stubs.CloseResponse) (err error) {
	closeDown <- true
	return
}

func main() {

	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()

	rpc.Register(&Worker{})

	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		fmt.Println("error with net.Listen")
		fmt.Println(err)
	}

	go closeWorker()

	defer listener.Close()
	rpc.Accept(listener)
}
