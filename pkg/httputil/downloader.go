package httputil

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/mitchellh/ioprogress"
	"go.uber.org/zap"
)

// Download downloads the file with progress bar.
// The progress is written to the writer.
func Download(lg *zap.Logger, w io.Writer, ep string) (downloadedData []byte, err error) {
	var size int64
	size, err = getSize(lg, ep)
	if err != nil {
		lg.Info("downloading (unknown size)", zap.String("url", ep), zap.Error(err))
	} else {
		lg.Info("downloading", zap.String("url", ep), zap.String("content-length", humanize.Bytes(uint64(size))))
	}

	resp, err := http.Get(ep)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var reader io.Reader
	if size != 0 && w != nil {
		reader = &ioprogress.Reader{
			Reader:       resp.Body,
			Size:         size,
			DrawFunc:     ioprogress.DrawTerminalf(w, drawTextFormatBytes),
			DrawInterval: time.Second,
		}
	} else {
		reader = resp.Body
	}
	downloadedData, err = ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	lg.Info("downloaded", zap.String("url", ep), zap.String("size", humanize.Bytes(uint64(len(downloadedData)))))
	return downloadedData, nil
}

func drawTextFormatBytes(progress, total int64) string {
	return fmt.Sprintf("\t%s / %s", humanize.Bytes(uint64(progress)), humanize.Bytes(uint64(total)))
}

func getSize(lg *zap.Logger, u string) (size int64, err error) {
	var resp *http.Response
	for i := 0; i < 5; i++ {
		resp, err = http.Head(u)
		if err == nil && resp.Header.Get("Content-Length") != "" {
			break
		}
		lg.Warn("failed to get header; retrying", zap.Error(err))
		time.Sleep(time.Second)
	}
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	size, err = strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return 0, err
	}
	return size, err
}
