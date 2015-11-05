package transfer

type Progress string

type Config struct {
	ListenPort   int
	TransferRate int
}

type Transfer interface {
	UpdateProgress(Progress)
	Config() Config
	Filename() string
	LocalDirectory() string
	FullPath() string
}
