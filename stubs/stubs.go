package stubs

import (
	"uk.ac.bris.cs/gameoflife/util"
)

var Distributor = "GOL.Distributor"
var GetAliveCells = "GOL.GetAliveCells"
var GetCurrentState = "GOL.GetCurrentState"
var SignalStateChange = "GOL.SignalStateChange"
var PartNextState = "Worker.PartNextState"
var CloseDistributor = "GOL.CloseDistributor"
var CloseWorker = "Worker.CloseWorker"

type RequestGame struct {
	World        [][]byte
	WorkerIPs    []string
	StartingTurn int
	Turns        int
	Threads      int
	ImageWidth   int
	ImageHeight  int
}

type ResponseGame struct {
	AliveCells     []util.Cell
	TurnsCompleted int
	World          [][]byte
}

type AliveCellsRequest struct {
}

type AliveCellsResponse struct {
	Turn     int
	NumAlive int
}

type StateRequest struct {
}

type StateResponse struct {
	Turn  int
	World [][]byte
}

type StateChange struct {
	StateFlag int
}

type StateChangeResponse struct {
	Turn int
}

type PartNextStateRequest struct {
	Width     int
	Height    int
	WorldPart [][]byte
}

type PartNextStateResponse struct {
	World [][]byte
}

type CloseRequest struct {
}

type CloseResponse struct {
}
