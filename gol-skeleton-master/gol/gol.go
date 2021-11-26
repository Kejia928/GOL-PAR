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

	//	Put the missing channels in here.

	ioCommandChannel := make(chan ioCommand)
	ioIdle := make(chan bool)
	Output := make(chan uint8, p.ImageWidth*p.ImageHeight)
	Input := make(chan uint8, p.ImageWidth*p.ImageHeight)
	filename := make(chan string, 1)

	ioChannels := ioChannels{
		command:  ioCommandChannel,
		idle:     ioIdle,
		filename: filename,
		output:   Output,
		input:    Input,
	}
	go startIo(p, ioChannels)

	distributorChannels := distributorChannels{
		events:     events,
		ioCommand:  ioCommandChannel,
		ioIdle:     ioIdle,
		ioFilename: filename,
		ioOutput:   Output,
		ioInput:    Input,
		KeyPressed: keyPresses,
	}
	distributor(p, distributorChannels)
}
