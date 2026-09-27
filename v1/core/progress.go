package core

// Progress describes progress through a loading or rendering operation.
type Progress struct {
	Stage   string
	Ratio   float64
	Current int
	Total   int
	Message string
}

const (
	StageLoading   = "loading"
	StageDecoding  = "decoding"
	StageRendering = "rendering"
	StageComplete  = "complete"
	StageError     = "error"
)
