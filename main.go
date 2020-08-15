package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
)

const gdbPath string = "gSSURGO_DC.gdb/"
const masterTableFileName string = "a00000001"

type RasFields struct {
	MTolerance  float64
	XYTolerance float64
	ZOrig       float64
	MOrig       float64
	MScale      float64
	ZScale      float64
	XOrig       float64
	YOrig       float64
	XYScale     float64
	ZTolerance  float64
	HasM        bool
	HasZ        bool
	WKT         string
	Column      string
}

type Shape struct {
	YMax        float64
	XMax        float64
	XMin        float64
	YMin        float64
	MOrig       float64
	ZOrig       float64
	ZScale      float64
	MScale      float64
	XYScale     float64
	XOrig       float64
	YOrig       float64
	HasZ        bool
	HasM        bool
	MTolerance  float64
	ZTolerance  float64
	XYTolerance float64
	WKT         string
}

type Field struct {
	Name         string
	Alias        string
	Type         uint8
	Nullable     bool
	RasterFields RasFields
	Shp          Shape
}

type BaseTable struct {
	GdbTablePath, GdbTablxPath string
	GdbTable, GdbTablX         *os.File
	NFeaturesX                 uint32
	SizeTablxOffsets           uint32
	Fields                     []Field
	HasFlags                   bool
	NullableFields             int
	Flags                      []uint8
}

// NFeatures        uint32
// HeaderOffset     uint32
// HeaderLength     uint32
// LayerGeomType    uint8

func (bt *BaseTable) getFlags(f *os.File) {
	if bt.HasFlags {
		nRemainingFlags := bt.NullableFields
		for nRemainingFlags > 0 {
			temp := readByte(f)
			bt.Flags = append(bt.Flags, temp)
			nRemainingFlags -= 8
		}
	}
}

func (bt *BaseTable) skipField(fld *Field, iFieldForFlagTest uint8) bool {
	if bt.HasFlags && fld.Nullable {
		var test uint8 = (bt.Flags[iFieldForFlagTest>>3] & (1 << (iFieldForFlagTest % 8)))
		iFieldForFlagTest++
		return test != 0
	}
	return false
}

type RasterInfo struct {
	Name string
	ID   int
}

type MasterTable struct {
	BaseTab BaseTable
	Rasters []RasterInfo
}

type RasterBase struct {
	FileName        string
	BaseTab         BaseTable
	BlockWidth      int32
	BlockHeight     int32
	BandWidth       int32
	BandHeight      int32
	EMinX           float64
	EMinY           float64
	EMaxX           float64
	EMaxY           float64
	BlockOriginX    float64
	BlockOriginY    float64
	DataType        string
	CompressionType string
	BandTypes       []uint8
	GeoTransform    [6]float64
}

func bandTypeToDataTypeString(bandTypes []byte) string {
	switch {
	case bandTypes[2] == 0x08 && bandTypes[3] == 0x00: //00000000 00000100 00001000 00000000
		return "1bit"
	case bandTypes[2] == 0x20 && bandTypes[3] == 0x00: //00000000 00000100 00100000 00000000
		return "4bit"
	case bandTypes[2] == 0x41 && bandTypes[3] == 0x00: //00000000 00000100 01000001 00000000
		return "int8"
	case bandTypes[2] == 0x40 && bandTypes[3] == 0x00: //00000000 00000100 01000000 0000000
		return "uint8"
	case bandTypes[2] == 0x81 && bandTypes[3] == 0x00: //00000000 00000100 10000001 00000000
		return "int16"
	case bandTypes[2] == 0x80 && bandTypes[3] == 0x00: //00000000 00000100 10000000 00000000
		return "uint16"
	case bandTypes[2] == 0x01 && bandTypes[3] == 0x01: //00000000 00000100 00000001 00000001
		return "int32"
	case bandTypes[2] == 0x02 && bandTypes[3] == 0x01: //00000000 00000100 00000010 00000001
		return "float32"
	case bandTypes[2] == 0x00 && bandTypes[3] == 0x01: //00000000 00000100 00000000 00000001
		return "uint32"
	case bandTypes[2] == 0x00 && bandTypes[3] == 0x02: //00000000 00000100 00000000 00000010
		return "64bit"
	default:
		fmt.Println("Unrecognised band data type")
		panic(errors.New("Unrecognised band data type"))
	}
}

func bandTypeToCompressionTypeString(bandTypes []byte) string {
	switch {
	case bandTypes[1] == 0x00: //bandTypes = 0 0 2  1 00000000 00000000 00000010 00000001
		return "uncompressed"
	case bandTypes[1] == 0x04: //bandTypes = 0 4 2  1 00000000 00000100 00000010 00000001
		return "lz77"
	case bandTypes[1] == 0x08: //bandTypes = 0 8 40 0 00000000 00001000 01000000 00000000
		return "jpeg"
	case bandTypes[1] == 0x0C: //bandTypes = 0 c 81 0 00000000 00001100 10000001 00000000
		return "jpeg2000"
	default:
		fmt.Println("Unrecognised band compression type")
		panic(errors.New("Unrecognised band compression type"))
	}
}

type RasterProjection struct {
	FileName string
}

type RasterData struct {
	BaseTab BaseTable
	GeoData []interface{}
	MinPx   int
	MinPy   int
	MaxPx   int
	MaxPy   int
	RasBase RasterBase
}

// func pprintStruct(st interface{}) {
// 	s := reflect.ValueOf(st)
// 	typeOfI := s.Type()
// 	for i := 0; i < s.NumField(); i++ {
// 		f := s.Field(i)
// 		fmt.Printf("%d: %s %s = %v\n", i, typeOfI.Field(i).Name, f.Type(), f.Interface())
// 	}
// }

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func assert(condition bool) {
	if condition {
		return
	}
	panic("Assertion error.")
}

func readU32(f *os.File) uint32 {
	b := make([]byte, 4)
	n, err := f.Read(b)
	check(err)
	assert(n == 4)
	return binary.LittleEndian.Uint32(b)
}

func readByte(f *os.File) uint8 {
	b := make([]byte, 1)
	n, err := f.Read(b)
	check(err)
	assert(n == 1)
	return uint8(b[0])
}

func readBytes(f *os.File, size int) []byte {
	b := make([]byte, size)
	n, err := f.Read(b)
	check(err)
	assert(n == size)
	return b
}

func readInt16(f *os.File) int16 {
	b := make([]byte, 2)
	n, err := f.Read(b)
	check(err)
	assert(n == 2)
	bits := binary.LittleEndian.Uint16(b)
	return int16(bits)
}

func readInt32(f *os.File) int32 {
	b := make([]byte, 4)
	n, err := f.Read(b)
	check(err)
	assert(n == 4)
	bits := binary.LittleEndian.Uint32(b)
	return int32(bits)
}

func readFloat32(f *os.File) float32 {
	b := make([]byte, 4)
	n, err := f.Read(b)
	check(err)
	assert(n == 4)
	bits := binary.LittleEndian.Uint32(b)
	return math.Float32frombits(bits)
}

func readFloat64(f *os.File) float64 {
	b := make([]byte, 8)
	n, err := f.Read(b)
	check(err)
	assert(n == 8)
	bits := binary.LittleEndian.Uint64(b)
	return math.Float64frombits(bits)
}

func readVarUint(f *os.File) uint64 {
	shift := uint64(0)
	ret := uint64(0)
	for {
		b := readByte(f)
		ret |= ((uint64(b) & 0x7F) << shift)
		if (b & 0x80) == 0 {
			break
		}
		shift += 7
	}
	return ret
}

func getString(f *os.File, nb int) string { // default nbcar to -1
	var nbcar int
	if nb == -1 {
		nbcar = int(readByte(f))
	} else {
		nbcar = nb
	}
	fmt.Printf("nbcar = %d\n", nbcar)
	str := ""
	for j := 0; j < int(nbcar); j++ {
		str += fmt.Sprintf("%c", readByte(f))
		f.Seek(1, 1)
	}
	return str
}

func newBaseTable(gdbFilePath string) BaseTable {
	tablePath := gdbFilePath + masterTableFileName + ".gdbtable"
	tablxPath := gdbFilePath + masterTableFileName + ".gdbtablx"
	gdbtablx, err := os.Open(tablxPath)
	check(err)
	defer gdbtablx.Close()

	gdbtablx.Seek(4, 0)
	num1024Blocks := readU32(gdbtablx)
	numFeaturesX := readU32(gdbtablx)

	if num1024Blocks == 0 {
		assert(numFeaturesX == 0)
	} else {
		assert(numFeaturesX >= 0)
	}
	sizeTablxOffsets := readU32(gdbtablx)

	gdbtable, err := os.Open(tablePath)
	check(err)
	defer gdbtable.Close()

	gdbtable.Seek(4, 0)
	readU32(gdbtable) // numFeatures

	gdbtable.Seek(32, 0)
	headerOff := readU32(gdbtable)

	gdbtable.Seek(int64(headerOff), 0)
	readU32(gdbtable) // headerLen

	gdbtable.Seek(4, 1)
	readByte(gdbtable) // layGeomType

	gdbtable.Seek(3, 1)
	numFields := int(readByte(gdbtable))
	numFields += int(readByte(gdbtable)) * 256

	hasFlags := false
	nullableFields := 0

	flds := make([]Field, 0)
	for i := 0; i < numFields; i++ {
		// nbcar := -1
		fld := Field{}

		fld.Name = getString(gdbtable, -1)
		fld.Alias = getString(gdbtable, -1)
		fld.Type = readByte(gdbtable)
		fld.Nullable = true
		fmt.Printf("fld.Name = %v\n", fld.Name)
		fmt.Printf("fld.Alias = %v\n", fld.Alias)
		fmt.Printf("fld.Type = %v\n", fld.Type)

		switch fld.Type {

		case 6: // ObjecdID
			readByte(gdbtable) // magic_byte1
			readByte(gdbtable) // magic_byte2
			fld.Nullable = false

		case 7: // Shape
			readByte(gdbtable) // magic_byte1 // 0
			flag := readByte(gdbtable)
			if (flag & 1) == 0 {
				fld.Nullable = false
			}
			wktLen := int(readByte(gdbtable))
			fld.Shp.WKT = getString(gdbtable, wktLen/2)

			magicByte3 := readByte(gdbtable)

			fld.Shp.HasM = false
			fld.Shp.HasZ = false
			if magicByte3 == 5 {
				fld.Shp.HasZ = true
			}
			if magicByte3 == 7 {

				fld.Shp.HasM = true
				fld.Shp.HasZ = true
			}

			fld.Shp.XOrig = readFloat64(gdbtable)
			fld.Shp.YOrig = readFloat64(gdbtable)
			fld.Shp.XYScale = readFloat64(gdbtable)
			if fld.Shp.HasM {
				fld.Shp.MOrig = readFloat64(gdbtable)
				fld.Shp.MScale = readFloat64(gdbtable)
			}

			if fld.Shp.HasZ {
				fld.Shp.ZOrig = readFloat64(gdbtable)
				fld.Shp.ZScale = readFloat64(gdbtable)
			}
			fld.Shp.XYTolerance = readFloat64(gdbtable)
			if fld.Shp.HasM {
				fld.Shp.MTolerance = readFloat64(gdbtable)
			}
			if fld.Shp.HasZ {
				fld.Shp.ZTolerance = readFloat64(gdbtable)
			}

			fld.Shp.XMin = readFloat64(gdbtable)
			fld.Shp.YMin = readFloat64(gdbtable)
			fld.Shp.XMax = readFloat64(gdbtable)
			fld.Shp.YMax = readFloat64(gdbtable)

			//TODO: What is this doing?
			for {
				read5 := readBytes(gdbtable, 5)
				if read5[0] != 0 || (read5[1] != 1 && read5[1] != 2 && read5[1] != 3) || read5[2] != 0 || read5[3] != 0 || read5[4] != 0 {
					gdbtable.Seek(-5, 1)
					readFloat64(gdbtable) // datum
				} else {
					for i := 0; i < int(read5[1]); i++ {
						readFloat64(gdbtable) // datum
						break
					}
				}
			}

		case 4: // String
			readU32(gdbtable) // width
			flag := readByte(gdbtable)

			if (flag & 1) == 0 {
				fld.Nullable = false
			}

			defaultValueLength := readVarUint(gdbtable)
			if (flag&4) != 0 && defaultValueLength > 0 {
				gdbtable.Seek(int64(defaultValueLength), 1)
			}

		case 8: //TODO: What is this?
			gdbtable.Seek(1, 1)
			flag := readByte(gdbtable)
			if (flag & 1) == 0 {
				fld.Nullable = false
			}

		case 9: // Raster
			gdbtable.Seek(1, 1)
			flag := readByte(gdbtable)
			if (flag & 1) == 0 {
				fld.Nullable = false
			}

			fld.RasterFields.Column = getString(gdbtable, -1)

			wktLen := int(readByte(gdbtable))
			fld.RasterFields.WKT = getString(gdbtable, wktLen/2)
			// fmt.Println("WKT:", fld.RasterFields.WKT)

			magicByte3 := readByte(gdbtable)
			if magicByte3 > 0 {
				fld.RasterFields.HasM = false
				fld.RasterFields.HasZ = false

				if magicByte3 == 5 {
					fld.RasterFields.HasZ = true
				} else if magicByte3 == 7 {
					fld.RasterFields.HasM = true
					fld.RasterFields.HasZ = true
				}

				fld.RasterFields.XOrig = readFloat64(gdbtable)
				fld.RasterFields.YOrig = readFloat64(gdbtable)
				fld.RasterFields.XYScale = readFloat64(gdbtable)

				if fld.RasterFields.HasM {
					fld.RasterFields.MOrig = readFloat64(gdbtable)
					fld.RasterFields.MScale = readFloat64(gdbtable)
				}

				if fld.RasterFields.HasZ {
					fld.RasterFields.ZOrig = readFloat64(gdbtable)
					fld.RasterFields.ZScale = readFloat64(gdbtable)
				}

				fld.RasterFields.XYTolerance = readFloat64(gdbtable)
				if fld.RasterFields.HasM {
					fld.RasterFields.MTolerance = readFloat64(gdbtable)
				}
				if fld.RasterFields.HasZ {
					fld.RasterFields.ZTolerance = readFloat64(gdbtable)
				}
			}

			gdbtable.Seek(1, 1)

		case 10, 11, 12: //UUID or XML
			readByte(gdbtable) // width
			flag := readByte(gdbtable)
			if (flag & 1) == 0 {
				fld.Nullable = false
			}

		default:
			readByte(gdbtable) // width
			flag := readByte(gdbtable)
			if (flag & 1) == 0 {
				fld.Nullable = false
			}

			defaultValueLength := readByte(gdbtable)

			//TODO: What is this?
			if (flag & 4) != 0 {
				if fld.Type == 0 && defaultValueLength == 2 {
					readInt16(gdbtable) // default_value
				} else if fld.Type == 1 && defaultValueLength == 4 {
					readInt32(gdbtable) // default_value
				} else if fld.Type == 2 && defaultValueLength == 4 {
					readFloat32(gdbtable) // default_value
				} else if fld.Type == 3 && defaultValueLength == 8 {
					readFloat64(gdbtable) // default_value
				} else if fld.Type == 5 && defaultValueLength == 8 {
					readFloat64(gdbtable) // default_value
				} else {
					gdbtable.Seek(int64(defaultValueLength), 1)
				}
			}
		}

		if fld.Nullable {
			hasFlags = true
			nullableFields++
		}

		if fld.Type != 6 {
			flds = append(flds, fld)
		}

	}

	return BaseTable{
		tablePath,
		tablxPath,
		gdbtable,
		gdbtablx,
		numFeaturesX,
		sizeTablxOffsets,
		flds,
		hasFlags,
		nullableFields,
		make([]uint8, 0)}
}

func newMasterTable(bt *BaseTable) {}

func main() {

	bt := newBaseTable(gdbPath)
	// pprintStruct(bt)
	fmt.Printf("%#v\n", bt)

}
