package stubs

var GolHandler = "GolOperations.Evolve"

type Response struct {
	FinalWorld     [][]uint8
	CompletedTurns int
}

type Request struct {
	World       [][]uint8
	NewWorld    [][]uint8
	ImageHeight int
	ImageWidth  int
	Turns       int
}
