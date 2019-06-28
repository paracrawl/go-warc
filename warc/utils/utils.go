package utils

/*
	Copyright (C) 2015  Wolfgang Meyers

    This program is free software; you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation; either version 2 of the License, or
    (at your option) any later version.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License along
    with this program; if not, write to the Free Software Foundation, Inc.,
    51 Franklin Street, Fifth Floor, Boston, MA 02110-1301 USA.
*/
import (
	"bytes"
	"errors"
	"io"
	"math"
	"strings"
)

// Provides map-like behavior with case-insensitive keys
type CIStringMap struct {
	m map[string]string
}

func NewCIStringMap() *CIStringMap {
	return &CIStringMap{m: map[string]string{}}
}

func (mm *CIStringMap) Get(key string) (string, bool) {
	result, exists := mm.m[strings.ToLower(key)]
	return result, exists
}

func (mm *CIStringMap) Set(key string, value string) {
	mm.m[strings.ToLower(key)] = value
}

func (mm *CIStringMap) Delete(key string) {
	delete(mm.m, strings.ToLower(key))
}

func (mm *CIStringMap) Update(m map[string]string) {
	for key, value := range m {
		mm.m[strings.ToLower(key)] = value
	}
}

func (mm *CIStringMap) Keys() []string {
	result := make([]string, len(mm.m))
	i := 0
	for key, _ := range mm.m {
		result[i] = key
		i++
	}
	return result
}

func (mm *CIStringMap) Items(callback func(string, string)) {
	for key, value := range mm.m {
		callback(key, value)
	}
}

// File interface over a part of a file
type FilePart struct {
	fileobj  io.Reader
	filedata []byte // The contents of the file part are captured on instantiation
	length   int
	offset   int
	buf      []byte
}

// Creates a new FilePart object
func NewFilePart(fileobj io.Reader, length int) (*FilePart, error) {
	// impose an arbitrary 16M limit on file size
	if length > (2<<23) {
		length = 2<<23
	}

	filePart := &FilePart{
		fileobj: fileobj,
		length:  length,
		offset:  0,
		buf:     []byte{},
	}

	// Fix for thread-safety: fully read the contents of the FilePart
	// initially and put the contents in the buffer. This allows the
	// contents to be used by a different thread, freeing up the underlying
	// reader.
	buf, err := filePart.Read(-1)
	if err != nil && err.Error() != "EOF" {
		return nil, err
	}
	for len(buf) < length {
		tmp, err := filePart.Read(-1)
		if err != nil {
			break
		}
		buf = append(buf, tmp...)
	}

	filePart.offset = 0
	filePart.filedata = buf
	filePart.fileobj = bytes.NewBuffer(buf)
	return filePart, nil
}

// GetData returns the data that was cached from the
// initial read of the FilePart during instantiation.
func (fp *FilePart) GetData() []byte {
	return fp.filedata
}

// reads up until the size specified
func (fp *FilePart) Read(size int) ([]byte, error) {
	if size == -1 {
		return fp.read(fp.length)
	} else {
		return fp.read(size)
	}
}

func (fp *FilePart) read(size int) ([]byte, error) {
	var content []byte
	if len(fp.buf) >= size {
		content = fp.buf[:size]
		fp.buf = fp.buf[size:]
	} else {
		size = int(math.Min(float64(size), float64(fp.length-fp.offset-len(fp.buf))))
		tmp := make([]byte, size)
		// if this read doesn't succeed, that's ok
		// because the buffer might still have content
		numRead, _ := fp.fileobj.Read(tmp)
		//		if err != nil {
		//			return nil, err
		//		}
		tmp = tmp[:numRead]
		content = append(fp.buf, tmp...)
		fp.buf = []byte{}
	}
	fp.offset += len(content)
	if len(content) == 0 {
		return nil, errors.New("EOF")
	} else {
		return content, nil
	}

}

// backs up the reader to the beginning of the content
func (fp *FilePart) unread(content []byte) {
	fp.buf = append(content, fp.buf...)
	fp.offset -= len(content)
}

// Reads a single line of content
func (fp *FilePart) ReadLine() ([]byte, error) {
	result := []byte{}
	chunk, err := fp.read(1024)
	if err != nil {
		return nil, err
	}

	for findNewline(chunk) == -1 {
		result = append(result, chunk...)
		chunk, err = fp.read(1024)
		if err != nil && err.Error() == "EOF" {
			chunk = []byte{}
			break
		}
	}
	i := findNewline(chunk)
	if i != -1 {
		fp.unread(chunk[i+1:])
		chunk = chunk[:i+1]
	}
	result = append(result, chunk...)
	return result, nil
}

// Iterates and invokes the callback function for each line
func (fp *FilePart) Iterate(callback func([]byte)) {
	line, err := fp.ReadLine()
	if err != nil {
		return
	}
	for err == nil {
		callback(line)
		line, err = fp.ReadLine()
	}
}

func (fp *FilePart) GetReader() io.Reader {
	return fp.fileobj
}

func (fp *FilePart) GetLength() int {
	return fp.length
}

func findNewline(chunk []byte) int {
	return bytes.IndexByte(chunk, '\n')
}
