package main

import (
	"flag"
	"math/rand"
	"net"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

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

func calculateNextState(imageHeight, imageWidth int, world, resultWorld [][]uint8) {
	/*resultWorld := make([][]byte, imageHeight)
	for i := range resultWorld {
		resultWorld[i] = make([]byte, imageWidth)
	}*/
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
}

type GolOperations struct {
}

func (s *GolOperations) Evolve(req stubs.Request, res *stubs.Response) (err error) {
	world := req.World
	newWorld := req.NewWorld
	turn := 0
	for turn < req.Turns {
		calculateNextState(req.ImageHeight, req.ImageWidth, world, newWorld)
		world, newWorld = newWorld, world
		turn++
	}
	res.CompletedTurns = turn
	res.FinalWorld = world
	return
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	rpc.Register(&GolOperations{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
