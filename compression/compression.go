// Package compression provides message compression/decompression functionality for go-rabbitmq.
//
// This package implements various compression algorithms that can be used to reduce
// message size before publishing to RabbitMQ. Compression is particularly useful for
// large messages or high-throughput scenarios where bandwidth is a concern.
//
// Example usage:
//
//	import "github.com/cloudresty/go-rabbitmq/compression"
//
//	// Create a gzip compressor with 1KB threshold and default compression level
//	compressor := compression.NewGzip(1024, compression.DefaultLevel)
//
//	// Use with publisher
//	publisher, err := client.NewPublisher(
//		rabbitmq.WithCompression(compressor),
//	)
package compression

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"sync"

	"github.com/cloudresty/go-rabbitmq"
)

// Compression level constants for convenience
const (
	BestSpeed       = gzip.BestSpeed       // 1
	BestCompression = gzip.BestCompression // 9
	DefaultLevel    = gzip.DefaultCompression
	NoCompression   = gzip.NoCompression // 0
)

// bufferPool provides reusable buffers to reduce GC pressure during compression
// Each Get() returns a *bytes.Buffer that should be Reset() and Put() back after use
var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

//
// Gzip
//

// Gzip implements rabbitmq.MessageCompressor using gzip compression
type Gzip struct {
	threshold int
	level     int
	// writerPool is a per-level writer pool for this compressor instance
	// This ensures writers with the correct level are reused
	writerPool sync.Pool
}

// NewGzip creates a new gzip compressor with specified threshold and compression level
func NewGzip(threshold int, level int) rabbitmq.MessageCompressor {
	if threshold <= 0 {
		threshold = 1024 // Default 1KB threshold
	}
	if level < gzip.BestSpeed || level > gzip.BestCompression {
		level = gzip.DefaultCompression
	}
	g := &Gzip{
		threshold: threshold,
		level:     level,
	}
	// Initialize per-instance writer pool with correct compression level
	g.writerPool = sync.Pool{
		New: func() any {
			w, _ := gzip.NewWriterLevel(nil, g.level)
			return w
		},
	}
	return g
}

// Algorithm returns the compression algorithm name
func (g *Gzip) Algorithm() string {
	return "gzip"
}

// Threshold returns the minimum size threshold for compression
func (g *Gzip) Threshold() int {
	return g.threshold
}

// Compress compresses the data using gzip if it exceeds the threshold
// Uses sync.Pool for buffer and writer reuse to reduce GC pressure
func (g *Gzip) Compress(data []byte) ([]byte, error) {
	if len(data) < g.threshold {
		return data, nil // Don't compress small messages
	}

	// Get buffer from pool and ensure it's reset
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()

	// Get writer from pool and reset to use new buffer
	writer := g.writerPool.Get().(*gzip.Writer)
	writer.Reset(buf)

	if _, err := writer.Write(data); err != nil {
		_ = writer.Close() // Ignore close error when write fails
		g.writerPool.Put(writer)
		bufferPool.Put(buf)
		return nil, fmt.Errorf("failed to write compressed data: %w", err)
	}

	if err := writer.Close(); err != nil {
		g.writerPool.Put(writer)
		bufferPool.Put(buf)
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	// Return writer to pool after successful compression
	g.writerPool.Put(writer)

	// Copy compressed data before returning buffer to pool
	compressed := make([]byte, buf.Len())
	copy(compressed, buf.Bytes())
	bufferPool.Put(buf)

	// Only return compressed data if it's actually smaller
	if len(compressed) < len(data) {
		return compressed, nil
	}
	return data, nil
}

// Recommended strict implementation for Gzip.Decompress
func (g *Gzip) Decompress(data []byte) ([]byte, error) {
	// Create a new reader. If this fails, the data is not valid gzip.
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		// Return a descriptive error instead of silently failing.
		return nil, fmt.Errorf("data does not appear to be valid gzip: %w", err)
	}
	defer func() {
		_ = reader.Close() // Ignore close error in defer
	}()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress gzip data: %w", err)
	}

	return decompressed, nil
}

//
// Zlib
//

// Zlib implements Compressor using zlib compression
type Zlib struct {
	threshold int
	level     int
	// writerPool is a per-level writer pool for this compressor instance
	// This ensures writers with the correct level are reused
	writerPool sync.Pool
}

// NewZlib creates a new zlib compressor with specified threshold and compression level
func NewZlib(threshold int, level int) rabbitmq.MessageCompressor {
	if threshold <= 0 {
		threshold = 1024 // Default 1KB threshold
	}
	if level < zlib.BestSpeed || level > zlib.BestCompression {
		level = zlib.DefaultCompression
	}
	z := &Zlib{
		threshold: threshold,
		level:     level,
	}
	// Initialize per-instance writer pool with correct compression level
	z.writerPool = sync.Pool{
		New: func() any {
			w, _ := zlib.NewWriterLevel(nil, z.level)
			return w
		},
	}
	return z
}

// Algorithm returns the compression algorithm name
func (z *Zlib) Algorithm() string {
	return "zlib"
}

// Threshold returns the minimum size threshold for compression
func (z *Zlib) Threshold() int {
	return z.threshold
}

// Compress compresses the data using zlib if it exceeds the threshold
// Uses sync.Pool for buffer and writer reuse to reduce GC pressure
func (z *Zlib) Compress(data []byte) ([]byte, error) {
	if len(data) < z.threshold {
		return data, nil
	}

	// Get buffer from pool and ensure it's reset
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()

	// Get writer from pool and reset to use new buffer
	writer := z.writerPool.Get().(*zlib.Writer)
	writer.Reset(buf)

	if _, err := writer.Write(data); err != nil {
		_ = writer.Close() // Ignore close error when write fails
		z.writerPool.Put(writer)
		bufferPool.Put(buf)
		return nil, fmt.Errorf("failed to write compressed data: %w", err)
	}

	if err := writer.Close(); err != nil {
		z.writerPool.Put(writer)
		bufferPool.Put(buf)
		return nil, fmt.Errorf("failed to close zlib writer: %w", err)
	}

	// Return writer to pool after successful compression
	z.writerPool.Put(writer)

	// Copy compressed data before returning buffer to pool
	compressed := make([]byte, buf.Len())
	copy(compressed, buf.Bytes())
	bufferPool.Put(buf)

	// Only return compressed data if it's actually smaller
	if len(compressed) < len(data) {
		return compressed, nil
	}
	return data, nil
}

// Decompress decompresses zlib data with strict validation
func (z *Zlib) Decompress(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		// Return a descriptive error.
		return nil, fmt.Errorf("data does not appear to be valid zlib: %w", err)
	}
	defer func() {
		_ = reader.Close() // Ignore close error in defer
	}()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress zlib data: %w", err)
	}

	return decompressed, nil
}

// Nop is a no-operation compressor for testing and development
type Nop struct{}

// NewNop creates a new no-operation compressor
func NewNop() rabbitmq.MessageCompressor {
	return &Nop{}
}

// Compress returns data unchanged
func (n *Nop) Compress(data []byte) ([]byte, error) {
	return data, nil
}

// Decompress returns data unchanged
func (n *Nop) Decompress(data []byte) ([]byte, error) {
	return data, nil
}

// Algorithm returns "none"
func (n *Nop) Algorithm() string {
	return "none"
}

// Threshold returns 0
func (n *Nop) Threshold() int {
	return 0
}
