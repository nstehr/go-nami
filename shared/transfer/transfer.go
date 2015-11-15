package transfer

type Progress string

const (
	defaultTransferRate = 650000000 //bits per second
	defaultBlockSize    = 1024      //in bytes
	defaultErrorRate    = 7500      //threshhold error rate (% x 1000)
	defaultSlowerNum    = 25        //numerator in the slowdown factor
	defaultSlowerDen    = 24        //denominator in the slowdown factor
	defaultFasterNum    = 5         //numerator in the speedup factor
	defaultFasterDen    = 6         //denominator in the speedup factor
)

type Config struct {
	ListenPort   int
	TransferRate int   //bits per second
	BlockSize    int64 //in bytes
	ErrorRate    int   //threshhold error rate (% x 1000)
	SlowerNum    int   //numerator in the slowdown factor
	SlowerDen    int   //denominator in the slowdown factor
	FasterNum    int   //numerator in the speedup factor
	FasterDen    int   //denominator in the speedup factor
}

func NewConfig() Config {
	return Config{TransferRate: defaultTransferRate,
		BlockSize: defaultBlockSize,
		ErrorRate: defaultErrorRate,
		SlowerNum: defaultSlowerNum,
		SlowerDen: defaultSlowerDen,
		FasterNum: defaultFasterNum,
		FasterDen: defaultFasterDen}

}

type Transfer interface {
	UpdateProgress(Progress)
	Config() Config
	Filename() string
	LocalDirectory() string
	FullPath() string
}
