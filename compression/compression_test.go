package compression

import (
	"bytes"
	"testing"
)

func TestGzipCompression(t *testing.T) {
	compressor := NewGzip(10, DefaultLevel)

	// Test with data below threshold
	smallData := []byte("small")
	compressed, err := compressor.Compress(smallData)
	if err != nil {
		t.Fatalf("Compression failed: %v", err)
	}
	if !bytes.Equal(compressed, smallData) {
		t.Error("Small data should not be compressed")
	}

	// Test with data above threshold
	largeData := bytes.Repeat([]byte("This is a test message that should be compressed. "), 100)
	compressed, err = compressor.Compress(largeData)
	if err != nil {
		t.Fatalf("Compression failed: %v", err)
	}

	// Decompress and verify
	decompressed, err := compressor.Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompression failed: %v", err)
	}

	if !bytes.Equal(decompressed, largeData) {
		t.Error("Decompressed data doesn't match original")
	}

	// Test algorithm name
	if compressor.Algorithm() != "gzip" {
		t.Errorf("Expected algorithm 'gzip', got %s", compressor.Algorithm())
	}
}

func TestZlibCompression(t *testing.T) {
	compressor := NewZlib(10, DefaultLevel)

	// Test with data above threshold
	largeData := bytes.Repeat([]byte("This is a test message for zlib compression. "), 100)
	compressed, err := compressor.Compress(largeData)
	if err != nil {
		t.Fatalf("Compression failed: %v", err)
	}

	// Decompress and verify
	decompressed, err := compressor.Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompression failed: %v", err)
	}

	if !bytes.Equal(decompressed, largeData) {
		t.Error("Decompressed data doesn't match original")
	}

	// Test algorithm name
	if compressor.Algorithm() != "zlib" {
		t.Errorf("Expected algorithm 'zlib', got %s", compressor.Algorithm())
	}
}

func TestNopCompressor(t *testing.T) {
	compressor := NewNop()

	data := []byte("test data")
	compressed, err := compressor.Compress(data)
	if err != nil {
		t.Fatalf("Nop compression failed: %v", err)
	}

	if !bytes.Equal(compressed, data) {
		t.Error("Nop compressor should return data unchanged")
	}

	decompressed, err := compressor.Decompress(data)
	if err != nil {
		t.Fatalf("Nop decompression failed: %v", err)
	}

	if !bytes.Equal(decompressed, data) {
		t.Error("Nop decompressor should return data unchanged")
	}

	if compressor.Algorithm() != "none" {
		t.Errorf("Expected algorithm 'none', got %s", compressor.Algorithm())
	}

	if compressor.Threshold() != 0 {
		t.Errorf("Expected threshold 0, got %d", compressor.Threshold())
	}
}

func BenchmarkGzipCompression(b *testing.B) {
	compressor := NewGzip(100, DefaultLevel)
	data := bytes.Repeat([]byte("This is a benchmark test for gzip compression efficiency. "), 1000)

	for b.Loop() {
		_, err := compressor.Compress(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkZlibCompression(b *testing.B) {
	compressor := NewZlib(100, DefaultLevel)
	data := bytes.Repeat([]byte("This is a benchmark test for zlib compression efficiency. "), 1000)

	for b.Loop() {
		_, err := compressor.Compress(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
