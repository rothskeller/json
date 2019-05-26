package json

import (
	"bufio"
	"compress/gzip"
	"io"
	"sync"
)

var bufferPool sync.Pool
var gzipPool sync.Pool

// NewGZipWriter returns a writer implementation that emits the JSON in gzipped
// form.
func NewGZipWriter(w io.Writer) Writer {
	gjw := new(gzipWriter)
	if bwi := bufferPool.Get(); bwi != nil {
		gjw.bw = bwi.(*bufio.Writer)
		gjw.bw.Reset(w)
	} else {
		gjw.bw = bufio.NewWriter(w)
	}
	if gwi := gzipPool.Get(); gwi != nil {
		gjw.gw = gwi.(*gzip.Writer)
		gjw.gw.Reset(gjw.bw)
	} else {
		gjw.gw = gzip.NewWriter(gjw.bw)
	}
	gjw.writer = NewWriter(gjw.gw).(*writer)
	return gjw
}

// gzipWriter is the implementation of Writer that creates the JSON in GZIP
// form.  It defers to the base writer for everything except creation and close.
type gzipWriter struct {
	*writer
	bw *bufio.Writer
	gw *gzip.Writer
}

// Close closes the GZipWriter.
func (gjw *gzipWriter) Close() {
	gjw.writer.Close()
	gjw.writer = nil
	gjw.gw.Close()
	gzipPool.Put(gjw.gw)
	gjw.gw = nil
	gjw.bw.Flush()
	bufferPool.Put(gjw.bw)
	gjw.bw = nil
}
