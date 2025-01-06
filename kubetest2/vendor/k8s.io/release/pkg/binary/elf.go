/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package binary

import (
	"bufio"
	debugelf "debug/elf"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"k8s.io/release/pkg/consts"
)

// ELFBinary abstracts a binary in ELF format
type ELFBinary struct {
	Header  *ELFHeader
	Options *Options
}

// ELFHeader abstracts the data we need from the elf header
type ELFHeader struct {
	WordFlag   uint8    // Flag: 32 or 64 bit binary
	_          uint8    // byte order
	_          uint8    // ELF version
	OSABI      uint8    // Binary Interface
	ABIVersion uint8    // ABI Version
	_          [7]uint8 // EI_PAD Zero padding
	EType      uint16   // Executable Type: Executable, relocatable, etc
	EMachine   uint16   // Machine architecture
}

// NewELFBinary opens a file and returns an ELF binary if it is one
func NewELFBinary(filePath string, opts *Options) (*ELFBinary, error) {
	header, err := GetELFHeader(filePath)
	if err != nil {
		return nil, fmt.Errorf("while trying to get ELF header from file: %w", err)
	}
	if header == nil {
		logrus.Debug("file is not an ELF binary")
		return nil, nil
	}

	return &ELFBinary{
		Header:  header,
		Options: opts,
	}, nil
}

// String returns the relevant info of the header as a string
func (eh *ELFHeader) String() string {
	return fmt.Sprintf("%s %dbit", eh.MachineType(), eh.WordLength())
}

// WordLength returns either 32 or 64 for 32bit or 64 bit architectures
func (eh *ELFHeader) WordLength() int {
	if eh.WordFlag == 1 {
		return 32
	}
	if eh.WordFlag == 2 {
		return 64
	}
	logrus.Warn("Cannot determine if ELF binary is 32 or 64 bit")
	return 0
}

// MachineType returns a string with the architecture moniker
func (eh *ELFHeader) MachineType() string {
	switch eh.EMachine {
	// 0x02	SPARC
	// 0x03	x86
	case 0x03:
		return consts.ArchitectureI386
	// 0x06	Intel MCU
	// 0x07	Intel 80860
	// 0x08	MIPS
	// 0x09	IBM_System/370
	// 0x0A	MIPS RS3000 Little-endian
	// 0x14	PowerPC
	case 0x14:
		return consts.ArchitecturePPC
	// 0x15	PowerPC (64-bit)
	case 0x15:
		return consts.ArchitecturePPC64
	// 0x16	S390, including S390x
	case 0x16:
		return consts.ArchitectureS390X
	// 0x28	ARM (up to ARMv7/Aarch32)
	case 0x28:
		return consts.ArchitectureARM
	// 0x3E	amd64
	case 0x3e:
		return consts.ArchitectureAMD64
	// 0xB7	ARM 64-bits (ARMv8/Aarch64)
	case 0xb7:
		return consts.ArchitectureARM64
	// 0xF3	RISC-V
	case 0xF3:
		return consts.ArchitectureRISCV
	}
	logrus.Warn("Unknown machine type in elf binary")
	return "arch unknown"
}

// GetELFHeader returns the header if the binary is and EF binary
func GetELFHeader(path string) (*ELFHeader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening binary for reading: %w", err)
	}
	defer f.Close()

	// Read the first 20 bytes of the binary, just enough of the
	// header for us to get the info we need:
	reader := bufio.NewReader(f)
	hBytes, err := reader.Peek(6)
	if err != nil {
		return nil, fmt.Errorf("reading the binary header: %w", err)
	}

	logrus.Debugf("Header bytes: %+v", hBytes)

	// Check we're dealing with an elf binary:
	if string(hBytes[1:4]) != "ELF" {
		logrus.Debug("Binary is not an ELF executable")
		return nil, nil
	}

	// Check if binary byte order is big or little endian
	var endianness binary.ByteOrder
	switch hBytes[5] {
	case 1:
		endianness = binary.LittleEndian

	case 2:
		endianness = binary.BigEndian

	default:
		return nil, fmt.Errorf("invalid endianness specified in elf binary: %w", err)
	}

	header := &ELFHeader{}
	if _, err := f.Seek(4, 0); err != nil {
		return nil, fmt.Errorf("seeking past the ELF magic bytes: %w", err)
	}
	if err := binary.Read(f, endianness, header); err != nil {
		return nil, fmt.Errorf("reading elf header from binary file: %w", err)
	}
	return header, nil
}

// Arch return the GOOS label of the binary
func (elf *ELFBinary) Arch() string {
	return elf.Header.MachineType()
}

// OS returns the GOOS label for the operating system
func (elf *ELFBinary) OS() string {
	return LINUX
}

// LinkMode returns the linking mode of the binary.
func (elf *ELFBinary) LinkMode() (LinkMode, error) {
	file, err := os.Open(elf.Options.Path)
	if err != nil {
		return LinkModeUnknown, fmt.Errorf("open binary path: %w", err)
	}

	elfFile, err := debugelf.NewFile(file)
	if err != nil {
		return LinkModeUnknown, fmt.Errorf("unable to parse elf: %w", err)
	}

	for _, programHeader := range elfFile.Progs {
		// If the elf program header refers to an interpreter, then the binary
		// is not statically linked. See `file` implementation reference:
		// https://github.com/file/file/blob/FILE5_36/src/readelf.c#L1581
		if programHeader.Type == debugelf.PT_INTERP {
			return LinkModeDynamic, nil
		}
	}

	return LinkModeStatic, nil
}
