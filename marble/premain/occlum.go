package premain

/*
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <fcntl.h>

typedef struct {
	uint8_t* report_data;
	uint32_t* quote_len;
	uint8_t* quote_buf;
} sgxioc_gen_dcap_quote_arg_t;
*/
import "C"

import (
	"crypto/sha256"
	"errors"
	"os"
	"syscall"
	"unsafe"
)

// ioctlGetQuoteSize holds the ioctl value for the SGX device to retrieve the size of the quote in bytes.
// Generated by the following macro: _IOR('s', 7, uint32_t)
const ioctlGetQuoteSize = 2147775239

// ioctlGenerateQuote holds the ioctl value for the SGX device to issue a quote.
// For the struct definition, see above.
// Generated by the following macro: _IOWR('s', 8, sgxioc_gen_dcap_quote_arg_t)
const ioctlGenerateQuote = 3222827784

type OcclumQuoteIssuer struct{}

// Issue issues a quote for remote attestation for a given message (usually a certificate)
func (OcclumQuoteIssuer) Issue(cert []byte) ([]byte, error) {
	// Open SGX device for ioctl() operations
	sgxDevice, err := os.OpenFile("/dev/sgx", os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer sgxDevice.Close()

	// Get supported quote size from SGX Device
	dcapQuoteSize, err := getQuoteSize(sgxDevice)
	if err != nil {
		return nil, err
	}

	// Generate raw quote
	quoteBytes, err := generateQuote(sgxDevice, dcapQuoteSize, cert)
	if err != nil {
		return nil, err
	}

	return prependOEHeaderToRawQuote(quoteBytes[:dcapQuoteSize]), nil
}

func getQuoteSize(sgxDevice *os.File) (uint32, error) {
	var dcapQuoteSize uint32

	// Retrieve size of quote via ioctl
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, sgxDevice.Fd(), ioctlGetQuoteSize, uintptr(unsafe.Pointer(&dcapQuoteSize)))
	if errno != 0 {
		return 0, errno
	}

	return dcapQuoteSize, nil
}

func generateQuote(sgxDevice *os.File, sgxQuoteSize uint32, cert []byte) ([]byte, error) {
	// Build our report

	// Create struct for quote generation
	dcapCQuoteSize := C.uint32_t(sgxQuoteSize)
	dcapQuoteBuffer := make([]byte, sgxQuoteSize)

	// Generate reportData: SHA-256 hash of input data
	hash := sha256.Sum256(cert)
	reportData := make([]byte, 64)
	bytesCopied := copy(reportData, hash[:])
	if bytesCopied != 32 {
		return nil, errors.New("too few bytes copied into report data for quote generation")
	}

	// Fill struct for quote generation
	dcapQuote := C.sgxioc_gen_dcap_quote_arg_t{
		report_data: (*C.uint8_t)(&reportData[0]),
		quote_len:   (*C.uint32_t)(&dcapCQuoteSize),
		quote_buf:   (*C.uint8_t)(&dcapQuoteBuffer[0]),
	}

	// Generate quote via ioctl
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, sgxDevice.Fd(), ioctlGenerateQuote, uintptr(unsafe.Pointer(&dcapQuote)))
	if errno != 0 {
		return nil, errno
	}

	return dcapQuoteBuffer, nil
}
