package hardforks

//go:generate go run ./gen/main.go -type=Fork -input=./forks.go
type Fork uint32

const (
	Regolith Fork = iota
	Delta
	Ecotone
	Fjord
	Granite
	Holocene
	Isthmus
	Interop
	Jovian
	// Add new hardforks here!
)
