package atomicGob

import (
	"crypto/md5"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"raft/terr"
	"sync"
)

var files = make(map[string]*GobFile) //this is a map of files to keep different files locked

/********* file object and methods ********************************************/
type GobFile struct {
	write   chan interface{}
	path    string
	file    *os.File    // the pointer to the gob storage
	fileBak *os.File    //the backup file for gob
	Data    interface{} //the Data is what you write out
	encoder *gob.Encoder
	decoder *gob.Decoder
	sync.RWMutex
	sync.WaitGroup
}

//adds a new file into files
func NewGobFile(path string, kind interface{}) *GobFile {
	var check interface{}
	if _, ok := files[path]; ok {
		//file already exists.
		return files[path]
	}
	f := GobFile{path: path}
	t, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	f.file = t
	terr.PrintError("New Gob File", err)
	f.fileBak, err = os.OpenFile(path+".bak", os.O_RDWR|os.O_CREATE, 0755)
	terr.PrintError("Backup Gob File", err)

	f.Data = kind //add the kind to the Data
	f.write = make(chan interface{}, 90)
	gob.Register(f.Data)
	files[path] = &f

	//check if a file is readable, if it is read will copy backup back into message in the event
	//of a munged write file
	f.Read(&check)
	return files[path]
}

func (f *GobFile) GetMD5() []byte {
	var thisHash []byte
	hash := md5.New()
	_, err := io.Copy(hash, f.file)
	terr.PrintError("MD5 hash file unopenable", err)
	if err != nil {
		return thisHash
	}
	return hash.Sum(nil)
}

/************ file modification functions *************************************/

//Write creates a file using a COW type writer to ensure all or nothing writing.
func (f *GobFile) Write() {
	f.Add(1)
	//f.Lock()
	f.write <- &f.Data
	//defer f.Unlock()
	defer f.Done()

	//make a backup copy
	err := copy(f.file, f.fileBak)
	terr.BreakError("Write Atomic Gob", err)

	f.file.Seek(0, 0)      // go back to the start of the file
	os.Truncate(f.path, 0) // reset the size of the file.
	f.encoder = gob.NewEncoder(f.file)

	err2 := f.encoder.Encode(<-f.write)
	terr.BreakError("Gob encode failure", err2)
	f.file.Sync()
}

func (f *GobFile) Close() {
	f.file.Close()
	f.fileBak.Close()
}

//Read a gob file.
//calls must use a type assertion to ensure return of the desired GOB.
func (f *GobFile) Read(RG *interface{}) error {
	f.Add(1)
	//var err error
	go func() {
		f.Lock()
		defer f.Done()
		defer f.Unlock()
		f.file.Seek(0, 0)
		//Try to load and read the existing gob file
		f.decoder = gob.NewDecoder(f.file)
		f.decoder.Decode(RG)
		terr.LogInfo(fmt.Sprintf("Primary Read Failure on %s, loading backup", f.path))
		if *RG != nil { // gob successfully retrieved.
			return
		}
		f.fileBak.Seek(0, 0)
		err := fileGob(f.fileBak, RG)
		if err == nil { // gob successfully retrieved from backup. Restore File to Location
			copy(f.fileBak, f.file)
			t, err := os.OpenFile(f.path, os.O_RDWR|os.O_CREATE, 0755)
			terr.BreakError("backup replace file Creation", err)
			f.file = t
		} else {
			terr.LogInfo("File Gob Unloadable")
		}
	}()
	return nil
}

func copy(in *os.File, out *os.File) (err error) {
	in.Seek(0, 0)
	out.Seek(0, 0)
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

//fileGob collects a gob array from a file
func fileGob(network io.Reader, RG *interface{}) error {
	dec := gob.NewDecoder(network) // Will read from network.
	err := dec.Decode(&RG)
	return err
}

//SendGob sends a gob message over an io.Writer interface
func SendGob(msg interface{}, network io.Writer) {
	m2 := msg
	//gob.Register(m2)
	enc := gob.NewEncoder(network) // Will write to network.
	err := enc.Encode(&m2)
	terr.PrintError("Gob encode failure", err)

}

//RecieveGob collects a gob from an interface
func RecieveGob(network io.Reader, expected interface{}) interface{} {
	var msg interface{}
	gob.Register(expected)
	dec := gob.NewDecoder(network) // Will read from network.
	err := dec.Decode(&msg)
	if err != nil {
		return nil
	}
	return msg
}
