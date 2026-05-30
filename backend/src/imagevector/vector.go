package imagevector

import (
	"context"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const Dimension = 64

func FromReader(reader io.Reader) ([]float32, error) {
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}
	return FromImage(img), nil
}

func FromImage(img image.Image) []float32 {
	bounds := img.Bounds()
	vector := make([]float32, Dimension)
	total := float32(0)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			ri := int((r >> 8) / 64)
			gi := int((g >> 8) / 64)
			bi := int((b >> 8) / 64)
			index := ri*16 + gi*4 + bi
			if index >= 0 && index < len(vector) {
				vector[index] += 1
				total += 1
			}
		}
	}
	if total == 0 {
		return vector
	}
	for i := range vector {
		vector[i] = vector[i] / total
	}
	return vector
}

func FromSource(ctx context.Context, source string) ([]float32, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, errors.New("empty image source")
	}
	if strings.HasPrefix(source, "/api/v1/assets/ecommerce_agent_dataset/") {
		source = filepath.Join("..", "quality", "data", "ecommerce_agent_dataset", strings.TrimPrefix(source, "/api/v1/assets/ecommerce_agent_dataset/"))
	}
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return nil, err
		}
		client := &http.Client{Timeout: 8 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		return FromReader(io.LimitReader(resp.Body, 10<<20))
	}
	file, err := os.Open(source)
	if err != nil && !filepath.IsAbs(source) {
		file, err = os.Open(filepath.Join("..", source))
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return FromReader(file)
}
