package gol

import (
	"fmt"
	"log"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
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

/*func calculateNextState(imageHeight, imageWidth int, world [][]byte) [][]byte {
	resultWorld := make([][]byte, imageHeight)
	for i := range resultWorld {
		resultWorld[i] = make([]byte, imageWidth)
	}
	//var sum int
	for y := 0; y < imageHeight; y++ {
		for x := 0; x < imageWidth; x++ {
			sum := (world[(y+imageHeight-1)%imageHeight][(x+imageWidth-1)%imageWidth] / 255) + (world[(y+imageHeight-1)%imageHeight][(x+imageWidth)%imageWidth] / 255) +
				(world[(y+imageHeight-1)%imageHeight][(x+imageWidth+1)%imageWidth] / 255) + (world[(y+imageHeight)%imageHeight][(x+imageWidth-1)%imageWidth] / 255) +
				(world[(y+imageHeight)%imageHeight][(x+imageWidth+1)%imageWidth] / 255) + (world[(y+imageHeight+1)%imageHeight][(x+imageWidth-1)%imageWidth] / 255) +
				(world[(y+imageHeight+1)%imageHeight][(x+imageWidth)%imageWidth] / 255) + (world[(y+imageHeight+1)%imageHeight][(x+imageWidth+1)%imageWidth] / 255)
			if world[y][x] == 255 {
				if sum < 2 {
					resultWorld[y][x] = 0
				} else if sum == 2 || sum == 3 {
					resultWorld[y][x] = 255
				} else {
					resultWorld[y][x] = 0
				}
			} else {
				if sum == 3 {
					resultWorld[y][x] = 255
				} else {
					resultWorld[y][x] = 0
				}
			}
		}
	}
	return resultWorld
}*/

func calculateAliveCells(imageHeight, imageWidth int, world [][]byte) []util.Cell {
	var aliveCells []util.Cell
	for y := 0; y < imageHeight; y++ {
		for x := 0; x < imageWidth; x++ {
			if world[y][x] == 255 {
				aliveCells = append(aliveCells, util.Cell{x, y})
			}
		}
	}
	return aliveCells
}

// CLIENT CODE

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPress <-chan rune) {

	// TODO: Create a 2D slice to store the world.
	world := make([][]uint8, p.ImageHeight)
	newWorld := make([][]uint8, p.ImageHeight)
	for i := range world {
		world[i] = make([]uint8, p.ImageWidth)
		newWorld[i] = make([]uint8, p.ImageWidth)
	}

	c.ioCommand <- ioInput // load initial state from input file
	// get file name in the format of img.width x img.height
	// source: taken from test go files
	c.ioFilename <- fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			world[y][x] = <-c.ioInput
		}
	}

	turn := 0
	c.events <- StateChange{turn, Executing}

	// TODO: Execute all turns of the Game of Life.
	/*for turn < p.Turns {
		newWorld = calculateNextState(p.ImageHeight, p.ImageWidth, world)
		world = newWorld
		turn++
	}*/
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	//
	//mutex := &sync.Mutex{}
	//done := make(chan bool)
	//hard coding the server addr
	server := "127.0.0.1:8030"

	client, err1 := rpc.Dial("tcp", server)
	if err1 != nil {
		log.Fatal("dialing:", err1)
	}
	defer client.Close()

	request := stubs.Request{World: world, NewWorld: newWorld, ImageHeight: p.ImageHeight, ImageWidth: p.ImageWidth, Turns: p.Turns}
	response := new(stubs.Response)
	err2 := client.Call(stubs.GolHandler, request, response)
	if err2 != nil {
		panic(err2)
	}

	//go func() {
	/*for {
		select {
		case <-a.Done:
			break
		case <-ticker.C:
			fmt.Println("2 seconds")
		}
	}*/
	//}()
	//
	//response := makeCall1(client, world, newWorld, p.ImageHeight, p.ImageWidth, p.Turns)
	// TODO: Report the final state using FinalTurnCompleteEvent.
	c.events <- FinalTurnComplete{response.CompletedTurns, calculateAliveCells(p.ImageHeight, p.ImageWidth, response.FinalWorld)}
	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	//done <- true
	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
