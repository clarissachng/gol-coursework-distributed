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
}

func calculateAliveCells(p Params, world [][]byte) []util.Cell {
	var aliveCells []util.Cell // initialise an empty aliveCell slice

	for y := 0; y < p.ImageHeight; y++ { // Iterate over the entire grid and collect coordinates of alive cells.
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 0xFF { // if alive, add them into the cell struct
				aliveCells = append(aliveCells, util.Cell{x, y})
			}
		}
	}
	return aliveCells
}

// helper func to output image file (current world state -> .pgm file.)
func outputPgmFile(c distributorChannels, world [][]byte, imageWidth, imageHeight, turn int) {
	c.ioCommand <- ioOutput
	c.ioFilename <- fmt.Sprintf("%dx%dx%d", imageWidth, imageHeight, turn) // taken from test go file
	for y := 0; y < imageHeight; y++ {
		for x := 0; x < imageWidth; x++ {
			c.ioOutput <- world[y][x] // send pixel data to output channel
		}
	}

	// Make sure to wait for I/O to finish before signaling completion
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
}

// helper func for counting the live adjacent cells
func countliveNeighbours(x, y int, world [][]uint8, p Params) int {
	count := 0

	for dy := -1; dy <= 1; dy++ { // -1 = down, 1 = up
		for dx := -1; dx <= 1; dx++ { // -1 = left, 1 = right
			if dx == 0 && dy == 0 { // skip the center cell (current cell)
				continue
			}

			// use modulo arithmetic to handle wrapping at the world edges.
			neighbourY := (y + dy + p.ImageHeight) % p.ImageHeight
			neighbourX := (x + dx + p.ImageWidth) % p.ImageWidth

			if world[neighbourY][neighbourX] == 0xFF { // count the alive neighbour cells
				count++ // increment if neighbour is alive
			}
		}
	}
	return count
}

// helper func to do the gol routine - apply rules for gol
// cell state update function
func golOperator(world uint8, liveNeighbours int) uint8 {
	if world == 0xFF { // Cell is currently alive
		if liveNeighbours < 2 {
			return 0x00 // Cell dies
		} else if liveNeighbours == 2 || liveNeighbours == 3 {
			return 0xFF
		} else {
			return 0x00 // Cell dies
		}
	} else { // Cell is currently dead
		if liveNeighbours == 3 {
			return 0xFF // Cell becomes alive
		} else {
			return 0x00 // Cell stays dead
		}
	}
}

// worker func for multi-threads (parallel processing of rows)
func worker(startY, endY int, p Params, world, newWorld [][]uint8, mutex *sync.Mutex, flippedCellsChan chan []util.Cell, done chan bool) {
	var flippedCells []util.Cell

	for y := startY; y < endY; y++ {
		row := make([]uint8, p.ImageWidth)
		for x := 0; x < p.ImageWidth; x++ {
			liveNeighbours := countliveNeighbours(x, y, world, p)
			newState := golOperator(world[y][x], liveNeighbours)
			if world[y][x] != newState {
				flippedCells = append(flippedCells, util.Cell{x, y})
			}
			row[x] = newState
		}

		mutex.Lock()
		newWorld[y] = row // Update newWorld with computed row
		mutex.Unlock()
	}

	flippedCellsChan <- flippedCells
	done <- true
}

// helper func to quit gol and output final state
func quitProgram(turn int, world [][]byte, c distributorChannels, p Params, ticker *time.Ticker) {
	ticker.Stop()
	c.events <- FinalTurnComplete{turn, calculateAliveCells(p, world)}

	outputPgmFile(c, world, p.ImageWidth, p.ImageHeight, turn) //output img

	c.events <- ImageOutputComplete{turn, fmt.Sprintf("%dx%dx%d", p.ImageWidth, p.ImageHeight, turn)}
	c.events <- StateChange{turn, Quitting}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
	// TODO: Create a 2D slice to store the world.
	world := make([][]uint8, p.ImageHeight)
	newWorld := make([][]uint8, p.ImageHeight)
	for i := range world {
		world[i] = make([]uint8, p.ImageWidth)
		newWorld[i] = make([]uint8, p.ImageWidth)
	}

	c.ioCommand <- ioInput                                            // load initial state from input file
	c.ioFilename <- fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight) // source: taken from test go files
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			world[y][x] = <-c.ioInput
		}
	}

	turn := 0
	mutex := &sync.Mutex{}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	initialAliveCells := calculateAliveCells(p, world)
	for _, cell := range initialAliveCells { // Track initial alive cells and send initial events
		c.events <- CellFlipped{turn, cell}
	}

	c.events <- StateChange{turn, Executing}

	done, quit := make(chan bool), make(chan bool)
	isPaused := false

	//ticker goroutine to send alive cell counts when paused
	go func() {
		for range ticker.C {
			if isPaused {
				mutex.Lock()
				aliveCells := calculateAliveCells(p, world)
				c.events <- AliveCellsCount{turn, len(aliveCells)}
				mutex.Unlock()
			}
		}
	}()

	go func() {
		for {
			select {
			case key := <-keyPresses:
				switch key {
				case 's': // Save current world state as image
					fmt.Println("'s' is pressed, saving pgm image")
					mutex.Lock()
					outputPgmFile(c, world, p.ImageHeight, p.ImageWidth, turn)
					c.events <- ImageOutputComplete{turn, fmt.Sprintf("%dx%dx%d", p.ImageWidth, p.ImageHeight, turn)}
					mutex.Unlock()
				case 'q': // Quit the program and save final state
					fmt.Println("'q' is pressed, quitting gol and saving pgm image")
					mutex.Lock()
					quitProgram(turn, world, c, p, ticker)
					mutex.Unlock()
					quit <- true
					return
				case 'p': // Toggle pause state
					mutex.Lock()
					isPaused = !isPaused

					state := Paused // Initially assume the state is Paused first
					if !isPaused {
						state = Executing
					}
					fmt.Println("'p' is pressed, toggling pause")
					c.events <- StateChange{turn, state}
					mutex.Unlock()
					//if !isPaused {
					//	fmt.Println("p is pressed, pausing")
					//	fmt.Println("Paused, current turn is", turn)
					//	mutex.Lock()
					//	isPaused = true
					//	c.events <- StateChange{turn, Paused}
					//	mutex.Unlock()
					//} else {
					//	fmt.Println("p is pressed, continuing")
					//	mutex.Lock()
					//	isPaused = false
					//	c.events <- StateChange{turn, Executing}
					//	mutex.Unlock()
					//}
				}
			case <-done:
				return

			}
		}
	}()

	// TODO: Execute all turns of the Game of Life.
	rowsPerThread := p.ImageHeight / p.Threads

GolLoop:
	for turn < p.Turns {
		// check if it's paused
		if isPaused {
			select {
			case <-done:
				break GolLoop
			case <-quit:
				break GolLoop
			default:
				continue
			}
		}

		//fmt.Println("no pause")

		select {
		case <-done:
			break GolLoop
		case <-quit:
			//fmt.Println("quit is true")
			break GolLoop // exit from distributor function
		default:
			//fmt.Println("quit is false")
			// Create worker threads to process each section of the world
			turnDone := make(chan bool, p.Threads)
			flippedCellsChan := make(chan []util.Cell, p.Threads)

			for i := 0; i < p.Threads; i++ {
				startY := i * rowsPerThread
				endY := startY + rowsPerThread
				if i == p.Threads-1 {
					endY = p.ImageHeight
				}
				go worker(startY, endY, p, world, newWorld, mutex, flippedCellsChan, turnDone)
			}

			// Collect flipped cells from each worker and complete turn
			var allFlippedCells []util.Cell
			for i := 0; i < p.Threads; i++ {
				flippedCells := <-flippedCellsChan
				allFlippedCells = append(allFlippedCells, flippedCells...)
				<-turnDone
			}
			c.events <- CellsFlipped{turn, allFlippedCells}

			// Swap the worlds and increment turn counter
			mutex.Lock()
			world, newWorld = newWorld, world
			mutex.Unlock()

			turn++
			c.events <- TurnComplete{turn}
			// send updated alive cells count after each turn
			aliveCells := calculateAliveCells(p, world)
			c.events <- AliveCellsCount{turn, len(aliveCells)}
		}
	}
	close(done)
	//TODO: Report the final state using FinalTurnCompleteEvent.
	quitProgram(turn, world, c, p, ticker)
	//Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
