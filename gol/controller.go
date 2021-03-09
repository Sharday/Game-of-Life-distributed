package gol

import (
	"fmt"
	"net/rpc"
	"os"
	"strings"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
)

var shutDown = false

func generateOutput(p Params, c distributorChannels, world [][]byte, turn int) {
	//put bytes of final world into outputBytes channel
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.outputBytes <- world[y][x]
		}
	}
	fileName := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, turn)
	c.ioCommand <- ioOutput //command to write pgm output image
	c.fileName <- fileName
	c.events <- ImageOutputComplete{CompletedTurns: turn, Filename: fileName}
}

type distributorChannels struct {
	events      chan<- Event
	ioCommand   chan<- ioCommand
	ioIdle      <-chan bool
	fileName    chan<- string
	outputBytes chan<- uint8
	inputBytes  <-chan uint8
}

func getAliveCells(ticker *time.Ticker, done chan bool, c distributorChannels, client *rpc.Client) {
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			request := stubs.AliveCellsRequest{}
			response := new(stubs.AliveCellsResponse)
			err := client.Call(stubs.GetAliveCells, request, response)
			if err != nil {
				fmt.Println("client.Call error in getAliveCells")
				fmt.Println(err)
			}

			c.events <- AliveCellsCount{CompletedTurns: response.Turn, CellsCount: response.NumAlive}

		}
	}
}

func changeState(flag int, action string, client *rpc.Client, newState State, c distributorChannels) {
	request := stubs.StateChange{StateFlag: flag}
	response := new(stubs.StateChangeResponse)
	err := client.Call(stubs.SignalStateChange, request, response)
	if err != nil {
		fmt.Printf("error with client.Call for %s\n", action)
		fmt.Println(err)
	}

	c.events <- StateChange{CompletedTurns: response.Turn, NewState: newState}
	if newState == Paused {
		fmt.Printf("Current turn: %d\n", response.Turn)
	}
}

func keyPresses(keyChan <-chan rune, p Params, c distributorChannels, client *rpc.Client) {
	for {
		select {
		case keyPress := <-keyChan:
			switch keyPress {
			case 's':
				request := stubs.StateRequest{}
				response := new(stubs.StateResponse)
				err := client.Call(stubs.GetCurrentState, request, response)
				if err != nil {
					fmt.Println("error with client.Call for quitting: ")
					fmt.Println(err)
				}
				generateOutput(p, c, response.World, response.Turn)
			case 'q':
				changeState(0, "quitting", client, Quitting, c)
				return
			case 'p':
				changeState(1, "pausing", client, Paused, c)
				for {
					keyPress = <-keyChan
					for keyPress != 'p' {
						keyPress = <-keyChan
					}
					fmt.Println("Continuing")
					changeState(2, "resuming", client, Executing, c)
					break
				}
			case 'k':
				changeState(3, "shutting down", client, Shutting, c)
				shutDown = true
				return
			}
		default:
			break
		}
	}

}

func controller(p Params, c distributorChannels, keyChan <-chan rune) {

	//command line environment variables - server, workers, new_game
	serverAddr := os.Getenv("server")
	workers := os.Getenv("workers")
	workerIPs := strings.Split(workers, ",")

	//connect to distributor
	client, err := rpc.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Println("error with client dialing rpc server")
		fmt.Println(err)
	}
	defer client.Close()

	var world [][]byte
	var turns int

	startTurn := 0

	var newGame bool
	if os.Getenv("new_game") == "true" {
		newGame = true
	} else {
		newGame = false
	}

	if newGame {
		fmt.Println("Starting new game...")
		//empty world
		initialWorld := make([][]byte, p.ImageHeight)
		for i := range initialWorld {
			initialWorld[i] = make([]byte, p.ImageWidth)
		}

		//read input file
		fileName := fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)
		c.ioCommand <- ioInput
		c.fileName <- fileName
		//read input image bytes to form initial world
		for y := 0; y < p.ImageHeight; y++ {
			for x := 0; x < p.ImageWidth; x++ {
				initialWorld[y][x] = <-c.inputBytes
			}
		}
		world = initialWorld
		turns = p.Turns
	} else { //continue current game
		req := stubs.StateRequest{}
		res := new(stubs.StateResponse)
		er := client.Call(stubs.GetCurrentState, req, res)
		if er != nil {
			fmt.Println("error with client.Call for getting current state: ")
			fmt.Println(err)
		}
		world = res.World
		turns = p.Turns - res.Turn
		startTurn = res.Turn + 1
		fmt.Printf("Continuing at turn %d...\n", startTurn)
	}

	ticker := time.NewTicker(2 * time.Second)
	done := make(chan bool)

	go getAliveCells(ticker, done, c, client)

	go keyPresses(keyChan, p, c, client)

	//call Distributor function on engine to execute remaining turns of game of life
	request := stubs.RequestGame{World: world, WorkerIPs: workerIPs, StartingTurn: startTurn, Turns: turns, Threads: p.Threads, ImageWidth: p.ImageWidth, ImageHeight: p.ImageHeight}
	response := new(stubs.ResponseGame)
	err = client.Call(stubs.Distributor, request, response)
	if err != nil {
		fmt.Println("client.Call error calling Distributor:")
		fmt.Println(err)
	}

	if shutDown {
		request := stubs.CloseRequest{}
		response := new(stubs.CloseResponse)
		err := client.Call(stubs.CloseDistributor, request, response)
		if err != nil {
			fmt.Println("error with client.Call for shutting down Distributor: ")
			fmt.Println(err)
		}
	}

	c.events <- FinalTurnComplete{CompletedTurns: response.TurnsCompleted, Alive: response.AliveCells}
	done <- true
	generateOutput(p, c, response.World, response.TurnsCompleted)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)

}
