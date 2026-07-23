package service

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"testing"
)

func TestConvertUploadedGLTFToGLBTempPacksExternalResources(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	gltf := `{
		"asset": {"version": "2.0"},
		"buffers": [{"uri": "mesh.bin", "byteLength": 4}],
		"bufferViews": [{"buffer": 0, "byteOffset": 0, "byteLength": 4}],
		"images": [{"uri": "textures/albedo.png"}]
	}`
	writePart(t, writer, "file", "demo/model.gltf", []byte(gltf))
	if err := writer.WriteField("file_path", "demo/model.gltf"); err != nil {
		t.Fatal(err)
	}
	writePart(t, writer, "assets", "demo/mesh.bin", []byte{1, 2, 3, 4})
	if err := writer.WriteField("asset_path", "demo/mesh.bin"); err != nil {
		t.Fatal(err)
	}
	writePart(t, writer, "assets", "demo/textures/albedo.png", []byte{0x89, 'P', 'N', 'G'})
	if err := writer.WriteField("asset_path", "demo/textures/albedo.png"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/modelos3d", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(1024 * 1024); err != nil {
		t.Fatal(err)
	}

	main := req.MultipartForm.File["file"][0]
	assets := req.MultipartForm.File["assets"]
	out, size, sha, err := convertUploadedGLTFToGLBTemp(main, req.MultipartForm.Value["file_path"][0], assets, req.MultipartForm.Value["asset_path"], t.TempDir(), 1024*1024)
	if err != nil {
		t.Fatalf("convert gltf: %v", err)
	}
	if size == 0 || sha == "" {
		t.Fatalf("expected output size and sha, got size=%d sha=%q", size, sha)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint32(data[0:4]); got != glbMagic {
		t.Fatalf("magic = %x, want %x", got, glbMagic)
	}
	jsonLen := binary.LittleEndian.Uint32(data[12:16])
	if got := binary.LittleEndian.Uint32(data[16:20]); got != jsonChunk {
		t.Fatalf("json chunk type = %x, want %x", got, jsonChunk)
	}

	var doc map[string]any
	if err := json.Unmarshal(bytes.TrimRight(data[20:20+jsonLen], " "), &doc); err != nil {
		t.Fatal(err)
	}
	images := doc["images"].([]any)
	image := images[0].(map[string]any)
	if _, ok := image["uri"]; ok {
		t.Fatalf("image uri should be embedded: %#v", image)
	}
	if image["mimeType"] != "image/png" {
		t.Fatalf("mimeType = %v, want image/png", image["mimeType"])
	}
	if _, ok := image["bufferView"]; !ok {
		t.Fatalf("image bufferView missing: %#v", image)
	}
}

func writePart(t *testing.T, w *multipart.Writer, field, name string, data []byte) {
	t.Helper()
	part, err := w.CreateFormFile(field, name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
}
