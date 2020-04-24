package httputil

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/mitchellh/ioprogress"
	"go.uber.org/zap"
)

// Read downloads the file with progress bar.
// The progress is written to the writer.
func Read(lg *zap.Logger, progressWriter io.Writer, downloadURL string) (data []byte, err error) {
	cli := &http.Client{Transport: httpFileTransport}
	rd, closeFunc, err := createReader(lg, cli, progressWriter, downloadURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		closeFunc()
	}()
	data, err = ioutil.ReadAll(rd)
	if err != nil {
		return nil, err
	}
	lg.Info("downloaded", zap.String("download-url", downloadURL), zap.String("size", humanize.Bytes(uint64(len(data)))))
	return data, nil
}

// ReadInsecure downloads the file with progress bar.
// The progress is written to the writer.
func ReadInsecure(lg *zap.Logger, progressWriter io.Writer, downloadURL string) (data []byte, err error) {
	cli := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}}
	rd, closeFunc, err := createReader(lg, cli, progressWriter, downloadURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		closeFunc()
	}()
	data, err = ioutil.ReadAll(rd)
	if err != nil {
		return nil, err
	}
	lg.Info("downloaded", zap.String("download-url", downloadURL), zap.String("size", humanize.Bytes(uint64(len(data)))))
	return data, nil
}

// Download downloads to a file.
func Download(lg *zap.Logger, progressWriter io.Writer, downloadURL string, fpath string) error {
	cli := &http.Client{Transport: httpFileTransport}
	rd, closeFunc, err := createReader(lg, cli, progressWriter, downloadURL)
	if err != nil {
		return err
	}
	defer func() {
		closeFunc()
	}()

	f, err := os.OpenFile(fpath, os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		f, err = os.Create(fpath)
		if err != nil {
			return err
		}
	}
	defer f.Close()

	var n int64
	n, err = io.Copy(f, rd)
	if err != nil {
		lg.Warn("download to file failed", zap.Error(err))
		return fmt.Errorf("failed to download %q (%v)", downloadURL, err)
	}
	lg.Info("downloaded",
		zap.String("download-url", downloadURL),
		zap.String("download-path", fpath),
		zap.String("size", humanize.Bytes(uint64(n))),
	)
	return nil
}

var httpFileTransport *http.Transport

func init() {
	httpFileTransport = new(http.Transport)
	httpFileTransport.RegisterProtocol("file", http.NewFileTransport(http.Dir("/")))
}

func createReader(lg *zap.Logger, cli *http.Client, progressWriter io.Writer, downloadURL string) (rd io.Reader, closeFunc func(), err error) {
	var size int64
	size, err = getSize(lg, cli, downloadURL)
	if err != nil {
		lg.Info("downloading (unknown size)", zap.String("download-url", downloadURL), zap.Error(err))
	} else {
		lg.Info("downloading", zap.String("download-url", downloadURL), zap.String("content-length", humanize.Bytes(uint64(size))))
	}

	resp, err := cli.Get(downloadURL)
	if err != nil {
		return nil, func() {}, err
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, func() {}, fmt.Errorf("%q returned %d", downloadURL, resp.StatusCode)
	}
	closeFunc = func() {
		resp.Body.Close()
	}
	if size != 0 && progressWriter != nil {
		rd = &ioprogress.Reader{
			Reader:       resp.Body,
			Size:         size,
			DrawFunc:     ioprogress.DrawTerminalf(progressWriter, drawTextFormatBytes),
			DrawInterval: time.Second,
		}
	} else {
		rd = resp.Body
	}
	return rd, closeFunc, nil
}

func drawTextFormatBytes(progress, total int64) string {
	return fmt.Sprintf("\t%s / %s", humanize.Bytes(uint64(progress)), humanize.Bytes(uint64(total)))
}

func getSize(lg *zap.Logger, cli *http.Client, downloadURL string) (size int64, err error) {
	length := ""
	for i := 0; i < 3; i++ {
		resp, err := cli.Head(downloadURL)
		if err == nil && resp.Header.Get("Content-Length") != "" {
			length = resp.Header.Get("Content-Length")
			resp.Body.Close()
			break
		}
		resp.Body.Close()
		lg.Warn("failed to get header; retrying", zap.Error(err))
		time.Sleep(time.Second)
	}
	if err != nil {
		return 0, err
	}

	size, err = strconv.ParseInt(length, 10, 64)
	if err != nil {
		return 0, err
	}
	return size, err
}
