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

const alive = 255
const dead = 0

func mod(x, m int) int {
	return (x + m) % m
}

func creatWorld(imageHeight, imageWidth int) [][]byte {
	world := make([][]byte, imageHeight)
	for i := range world {
		world[i] = make([]byte, imageWidth)
	}
	return world
}

func calculateNewState(p Params, world [][]byte, startY, endY, startX, endX, turn int) [][]byte {
	partWorld := creatWorld(endY-startY, endX-startX)
	if turn == -1 {
		for y := 0; y < endY-startY; y++ {
			for x := 0; x < endX; x++ {
				partWorld[y][x] = world[startY+y][x]
			}
		}
		return partWorld
	} else {
		for y := 0; y < endY-startY; y++ {
			for x := 0; x < endX; x++ {

				//calculate neighbours
				neighbours := 0
				for i := -1; i <= 1; i++ {
					for j := -1; j <= 1; j++ {
						if i != 0 || j != 0 {
							if world[mod(startY+y+i, p.ImageHeight)][mod(x+j, p.ImageWidth)] == alive {
								neighbours++
							}
						}
					}
				}

				//make new world
				if world[startY+y][x] == alive {
					if neighbours < 2 || neighbours > 3 {
						partWorld[y][x] = dead
					} else {
						partWorld[y][x] = alive
					}
				} else {
					if neighbours == 3 {
						partWorld[y][x] = alive
					} else {
						partWorld[y][x] = dead
					}
				}
			}
		}
		return partWorld
	}
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

func worker(p Params, world [][]byte, startY, endY, startX, endX, turn int, out chan<- [][]byte) {
	result := calculateNewState(p, world, startY, endY, startX, endX, turn)
	out <- result
}

func getNewWorld(p Params, out []chan [][]byte, Range int) [][]byte {
	newWorld := creatWorld(p.ImageHeight, p.ImageWidth)

	//go through all channel for workers
	for o := 0; o < p.Threads; o++ {
		//take out the data from each channel
		result := out[o]
		partWorld := <-result
		//put data into the new world
		if o == p.Threads-1 {
			for y := 0; y < p.ImageHeight-(Range*(p.Threads-1)); y++ {
				for x := 0; x < p.ImageWidth; x++ {
					newWorld[(Range*(p.Threads-1))+y][x] = partWorld[y][x]
				}
			}
		} else {
			for y := 0; y < Range; y++ {
				for x := 0; x < p.ImageWidth; x++ {
					newWorld[((Range)*o)+y][x] = partWorld[y][x]
				}
			}
		}

	}
	return newWorld
}

func outPutImage(p Params, world [][]byte, filename string, c distributorChannels) {
	c.ioCommand <- ioOutput
	c.ioFilename <- filename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			outPut := world[y][x]
			c.ioOutput <- outPut
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

	//Create a 2D slice to store the world.
	world := creatWorld(p.ImageHeight, p.ImageWidth)

	//read file
	inputImageName := fmt.Sprintf("%vx%v", p.ImageHeight, p.ImageWidth)
	c.ioFilename <- inputImageName

	//store the world
	c.ioCommand <- ioInput
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			input := <-c.ioInput
			world[y][x] = input
		}
	}

	turn := 0
	height := p.ImageHeight / p.Threads
	width := p.ImageWidth
	tickerFinished := make(chan bool)
	mutex := &sync.Mutex{}
	aliveNum := 0
	state := Executing

	go func() {
		for {
			key := <- c.KeyPressed
			if key == 'p' {
				if state == Executing {
					c.events <- StateChange{
						CompletedTurns: turn,
						NewState:       Paused,
					}
					state = Paused

				} else if state == Paused{
					fmt.Println("Continuing")
					c.events <- StateChange{
						CompletedTurns: turn,
						NewState:       Executing,
					}
					state = Executing
				}
			} else if key == 'q' {
				outPutImage(p, world, "pressed-q", c)
				c.events <- ImageOutputComplete{
					CompletedTurns: turn,
					Filename:       "pressed-q",
				}
				c.events <- StateChange{
					CompletedTurns: turn,
					NewState:       Quitting,
				}
				state = Quitting
				c.ioCommand <- ioCheckIdle
				<- c.ioIdle
				close(c.events)

			} else if key == 's' {
				outPutImage(p, world, "pressed-s", c)
				c.events <- ImageOutputComplete{
					CompletedTurns: turn,
					Filename:       "pressed-s",
				}
			}
		}

	}()

	//Ticker : every two second out put the alive cells
	go func() {

		for {
			ticker := time.NewTicker(2 * time.Second)
			select {
			case <-ticker.C:
				mutex.Lock()
				if state == Executing {
					c.events <- AliveCellsCount{CompletedTurns: turn, CellsCount: aliveNum}
				}
				mutex.Unlock()
			case <-tickerFinished:
				ticker.Stop()
				return
			}
		}

	}()

	//Execute all turns of the Game of Life.
	for {
		if turn >= p.Turns {
			break
		}
		//create channel
		out := make([]chan [][]byte, p.Threads)
		for i := 0; i < p.Threads; i++ {
			out[i] = make(chan [][]byte)
		}

		for thread := 0; thread < p.Threads; thread++ {
			if thread != p.Threads-1 {
				go worker(p, world, height*thread, height*(thread+1), 0, width, turn, out[thread])
			} else {
				go worker(p, world, height*thread, p.ImageHeight, 0, width, turn, out[thread])
			}
		}

		newWorld := getNewWorld(p, out, height)

		if state == Executing {
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
		}
		world = newWorld
		mutex.Lock()
		aliveNum = len(calculateAliveCells(p, world))
		if state == Executing {
			c.events <- TurnComplete{CompletedTurns: turn}
			turn++
		}
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
