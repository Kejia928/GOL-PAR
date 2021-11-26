package gol

import (
	"fmt"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	KeyPressed <-chan rune
}

type Task struct {
	World [][]byte
	Height int
	Width int
}

type Result struct {
	NewWorld [][]byte
	AliveNum int
}

// CreatWorld Creat a 2D slice
func CreatWorld(imageHeight, imageWidth int) [][]byte {
	world := make([][]byte, imageHeight)
	for i := range world {
		world[i] = make([]byte, imageWidth)
	}
	return world
}

// LoadWorld store the file to the world
func LoadWorld(p Params, c distributorChannels) [][]byte {
	world := CreatWorld(p.ImageHeight, p.ImageWidth)
	//read file
	inputImageName := fmt.Sprintf("%vx%v", p.ImageHeight, p.ImageWidth)
	c.ioFilename <- inputImageName
	//store the world
	c.ioCommand <- ioInput
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			world[y][x] = <-c.ioInput
		}
	}
	return world
}

// GetTaskForEachWorker is used to allocate the task to each worker
func GetTaskForEachWorker(p Params, world [][]byte, thread int, tasks []Task) []Task {
	height := p.ImageHeight/thread
	for t := 0; t < thread; t++ {
		tasks[t].Width = p.ImageWidth
		if t != thread-1 {
			tasks[t].Height = height
		} else {
			tasks[t].Height = p.ImageHeight-(height*(thread-1))
		}
		//separate the world
		for y := 0; y <= tasks[t].Height+1; y++ {
			for x := 0; x < tasks[t].Width; x++ {
				if (height*t-1+y) < 0 {
					tasks[t].World[y][x] = world[p.ImageHeight-1][x]
				} else if (height*t-1+y) > p.ImageHeight-1 {
					tasks[t].World[y][x] = world[0][x]
				} else {
					tasks[t].World[y][x] = world[(height*t-1)+y][x]
				}
			}
		}
	}
	return tasks
}

func Worker (task Task, result Result, out chan<- Result, group *sync.WaitGroup) {
	result = CalculateNewState(task, result)
	out <- result
	group.Done()
	return
}

func GetResult(p Params, out []chan Result, result Result) Result {
	Range := p.ImageHeight/p.Threads
	aliveNum := 0
	endY := Range
	//go through all channel for workers
	for o := 0; o < p.Threads; o++ {
		//take out the data from each channel
		res := <- out[o]
		aliveNum = aliveNum + res.AliveNum

		//put data into the new world
		if o == p.Threads-1 {
			endY = p.ImageHeight-(Range*(p.Threads-1))
		}
		for y := 0; y < endY; y++ {
			copy(result.NewWorld[((Range)*o)+y], res.NewWorld[y])
		}
	}
	result.AliveNum = aliveNum
	return result
}

func calculateAliveCells(p Params, world [][]byte) []util.Cell {
	var aliveCells []util.Cell
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == alive {
				aliveCells = append(aliveCells, util.Cell{X: x, Y: y})
			}
		}
	}
	return aliveCells
}

func outPutImage(p Params, world [][]byte, filename string, c distributorChannels) {
	c.ioCommand <- ioOutput
	c.ioFilename <- filename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}
}

func TickerAndKeyboardControl(p *Params, c *distributorChannels, turn, aliveNum *int, world *[][]byte, pause *bool, Execute *chan bool, mutex *sync.Mutex, tickerFinished *chan bool) {
	for {
		//Ticker : every two second out put the alive cells
		ticker := time.NewTicker(2 * time.Second)
		select {
		case key := <- c.KeyPressed:
			switch key {
			case 's':
				filename := fmt.Sprintf("%vx%vx%v-%s", p.ImageHeight, p.ImageWidth, turn, "Press-s")
				outPutImage(*p, *world, filename, *c)
				c.events <- ImageOutputComplete{CompletedTurns: *turn, Filename: filename}
			case 'q':
				*pause = true
				c.events <- StateChange{CompletedTurns: *turn, NewState: Quitting}
				filename := fmt.Sprintf("%vx%vx%v-%s", p.ImageHeight, p.ImageWidth, turn, "Press-q")
				outPutImage(*p, *world, filename, *c)
				c.ioCommand <- ioCheckIdle
				<- c.ioIdle
				c.events <- ImageOutputComplete{CompletedTurns: *turn, Filename: filename}
				close(c.events)
				return
			case 'p':
				if *pause {
					*Execute <- true
					*pause = false
					fmt.Println("Continuing")
					c.events <- StateChange{CompletedTurns: *turn, NewState: Executing}
				} else {
					*pause = true
					c.events <- StateChange{CompletedTurns: *turn, NewState: Paused}
				}
			case 'k':
				*pause = true
				c.events <- StateChange{CompletedTurns: *turn, NewState: Quitting}
				filename := fmt.Sprintf("%vx%vx%v-%s", p.ImageHeight, p.ImageWidth, turn, "Press-k")
				outPutImage(*p, *world, filename, *c)
				c.ioCommand <- ioCheckIdle
				<- c.ioIdle
				c.events <- ImageOutputComplete{CompletedTurns: *turn, Filename: filename}
				close(c.events)
				return
			}
		case <-ticker.C:
			mutex.Lock()
			if !*pause {
				c.events <- AliveCellsCount{CompletedTurns: *turn, CellsCount: *aliveNum}
			}
			mutex.Unlock()
		case <-*tickerFinished:
			ticker.Stop()
			return
		}
	}
}

func getFlippedCell(p Params, world, newWorld [][]byte) []util.Cell {
	var FlippedCell []util.Cell
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] != newWorld [y][x] {
				FlippedCell = append(FlippedCell, util.Cell{X: x, Y: y})
			}
		}
	}
	return FlippedCell
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	// Create a 2D slice to store the world.
	world := LoadWorld(p, c)
	newWorld := CreatWorld(p.ImageHeight, p.ImageWidth)
	turn := 0
	aliveNum := 0

	tasks := make([]Task, p.Threads)
	results := make([]Result, p.Threads)
	completeResult := Result{NewWorld: newWorld, AliveNum: 0}
	out := make([]chan Result, p.Threads)

	tickerFinished := make(chan bool)
	Execute := make(chan bool)
	pause := false
	mutex := &sync.Mutex{}
	waitGroup := &sync.WaitGroup{}

	// Initial
	for i := 0; i < p.Threads; i++{
		h := p.ImageHeight/p.Threads
		if i == p.Threads-1 { h = p.ImageHeight - ((p.Threads-1) * (p.ImageHeight / p.Threads)) }
		tasks[i].World = CreatWorld(h+2, p.ImageWidth) //add halo region in the world
		results[i].NewWorld = CreatWorld(h, p.ImageWidth)
		out[i] = make(chan Result, 1)
	}

	go TickerAndKeyboardControl(&p, &c, &turn, &aliveNum, &world, &pause, &Execute, mutex, &tickerFinished)

	// Execute all turns of the Game of Life.
	for ; turn < p.Turns; {
		// update the Task List to each worker
		tasks = GetTaskForEachWorker(p, world, p.Threads, tasks)

		// run all worker
		waitGroup.Add(p.Threads)

		for thread := 0; thread < p.Threads; thread++ {
			go Worker(tasks[thread], results[thread], out[thread], waitGroup)
		}
		waitGroup.Wait()

		completeResult = GetResult(p, out, completeResult)

		// KeyBoard Control
		if pause {
			<- Execute
		}

		var cells []util.Cell
		if turn == 0 {
			cells = calculateAliveCells(p, newWorld)
		} else {
			cells = getFlippedCell(p, world, newWorld)
		}
		for cell := 0; cell < len(cells); cell++ {
			c.events <- CellFlipped{
				CompletedTurns: turn,
				Cell: cells[cell],
			}
		}

		mutex.Lock()
		for y := 0; y < p.ImageHeight; y++ {
			copy(world[y], newWorld[y])
		}
		c.events <- TurnComplete{CompletedTurns: turn}
		aliveNum = completeResult.AliveNum
		turn++
		mutex.Unlock()

	}
	// Report the final state using FinalTurnCompleteEvent.
	tickerFinished <- true
	c.events <- FinalTurnComplete{
		CompletedTurns: turn,
		Alive:          calculateAliveCells(p, world),
	}
	outImageName := fmt.Sprintf("%vx%vx%v", p.ImageHeight, p.ImageWidth, turn)
	outPutImage(p, world, outImageName, c)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}
	
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
