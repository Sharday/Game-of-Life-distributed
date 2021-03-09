package gol

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events chan<- Event, keyPresses <-chan rune) {

	ioCommand := make(chan ioCommand)
	ioIdle := make(chan bool)
	fileName := make(chan string)
	outputBytes := make(chan uint8, p.ImageHeight*p.ImageWidth)
	inputBytes := make(chan uint8, p.ImageHeight*p.ImageWidth)

	distributorChannels := distributorChannels{
		events,
		ioCommand,
		ioIdle,
		fileName,
		outputBytes,
		inputBytes,
	}
	go controller(p, distributorChannels, keyPresses)

	ioChannels := ioChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: fileName,
		output:   outputBytes,
		input:    inputBytes,
	}
	go startIo(p, ioChannels)
}
