package thingsdb

import (
	"encoding/binary"
	"fmt"

	"github.com/vmihailenco/msgpack/v5"
)

// pkgHeaderSize is the size of a package header.
const pkgHeaderSize = 8

// PkgInitCapacity will be used as default capacity when allocating for a package.
const PkgInitCapacity = 8192

// pkg contains of a header and data.
type pkg struct {
	size uint32
	pid  uint16
	tp   uint8
	data []byte
}

// newPkg returns a poiter to a new pkg.
func newPkg(b []byte) (*pkg, error) {
	tp := b[6]
	check := b[7]

	if check != '\xff'^tp {
		return nil, fmt.Errorf("invalid checkbit")
	}

	return &pkg{
		size: binary.LittleEndian.Uint32(b),
		pid:  binary.LittleEndian.Uint16(b[4:]),
		tp:   tp,
		data: nil,
	}, nil
}

// setData sets package data
func (p *pkg) setData(b *[]byte, size uint32) {
	p.data = (*b)[pkgHeaderSize:size]
}

// pkgPackBin returns a byte array containing a header with serialized data.
func pkgPackBin(pid uint16, tp Proto, data []byte) []byte {

	datasz := len(data)

	pkgdata := make([]byte, pkgHeaderSize, pkgHeaderSize+datasz)
	pkgdata = append(pkgdata, data...)

	// set package length.
	binary.LittleEndian.PutUint32(pkgdata[0:], uint32(datasz))

	// set package pid.
	binary.LittleEndian.PutUint16(pkgdata[4:], pid)

	// set package type and check bit.
	pkgdata[6] = uint8(tp)
	pkgdata[7] = '\xff' ^ uint8(tp)

	return pkgdata
}

// pkgPack returns a byte array containing a header with serialized data.
func pkgPack(pid uint16, tp Proto, v interface{}) ([]byte, error) {

	data, err := msgpack.Marshal(v)
	if err != nil {
		return nil, err
	}

	data = pkgPackBin(pid, tp, data)

	return data, nil
}
