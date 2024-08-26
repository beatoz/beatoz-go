package libs

import (
	"github.com/tendermint/tendermint/libs/tempfile"
	"io"
	"os"
)

const DefaultSFilePerm = 0600

type FileIO struct {
	path string
	flag int
	perm os.FileMode
}

var _ io.Reader = &FileIO{}
var _ io.Writer = &FileIO{}

func NewFileWriter(path string) *FileIO {
	return &FileIO{
		path: path,
		flag: os.O_RDWR | os.O_CREATE | os.O_TRUNC,
		perm: DefaultSFilePerm,
	}
}

func NewFileReader(path string) *FileIO {
	return &FileIO{
		path: path,
		flag: os.O_RDONLY,
		perm: DefaultSFilePerm,
	}
}

func (fw *FileIO) Read(d []byte) (int, error) {
	f, err := os.OpenFile(fw.path, fw.flag, fw.perm)
	if err != nil {
		return 0, err
	}

	defer f.Close()

	return f.Read(d)
}

func (fw *FileIO) Write(d []byte) (int, error) {

	err := tempfile.WriteFileAtomic(fw.path, d, fw.perm)
	if err != nil {
		panic(err)
	}

	f, err := os.OpenFile(fw.path, fw.flag, fw.perm)
	if err != nil {
		return 0, err
	}

	defer f.Close()

	return f.Write(d)
}
