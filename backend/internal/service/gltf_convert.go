package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"gitlab.com/syncbox/backend/internal/domain"
)

const (
	glbMagic   uint32 = 0x46546C67
	glbVersion uint32 = 2
	jsonChunk  uint32 = 0x4E4F534A
	binChunk   uint32 = 0x004E4942
)

func convertUploadedGLTFToGLBTemp(main *multipart.FileHeader, mainPath string, assets []*multipart.FileHeader, assetPaths []string, outDir string, maxBytes int64) (string, int64, string, error) {
	if strings.TrimSpace(mainPath) == "" {
		mainPath = main.Filename
	}
	mainPath, err := cleanUploadPath(mainPath)
	if err != nil {
		return "", 0, "", err
	}
	baseDir := path.Dir(mainPath)
	if baseDir == "." {
		baseDir = ""
	}

	gltfBytes, err := readFileHeader(main, maxBytes+1)
	if err != nil {
		return "", 0, "", err
	}
	if int64(len(gltfBytes)) > maxBytes {
		return "", 0, "", domain.ErrModelTooLarge
	}

	doc, err := decodeGLTFJSON(gltfBytes)
	if err != nil {
		return "", 0, "", err
	}

	assetMap, err := buildAssetMap(assets, assetPaths)
	if err != nil {
		return "", 0, "", err
	}

	binTmp, err := os.CreateTemp(outDir, "gltf-bin-*.tmp")
	if err != nil {
		return "", 0, "", err
	}
	binTmpName := binTmp.Name()
	defer func() {
		_ = binTmp.Close()
		_ = os.Remove(binTmpName)
	}()

	bufferOffsets, binLen, err := packBuffers(doc, baseDir, assetMap, binTmp, maxBytes)
	if err != nil {
		return "", 0, "", err
	}
	if err := rewriteBufferViews(doc, bufferOffsets); err != nil {
		return "", 0, "", err
	}
	binLen, err = packImages(doc, baseDir, assetMap, binTmp, binLen, maxBytes)
	if err != nil {
		return "", 0, "", err
	}
	binLen, err = padFile4(binTmp, binLen, 0x00)
	if err != nil {
		return "", 0, "", err
	}
	if err := binTmp.Close(); err != nil {
		return "", 0, "", err
	}

	if binLen > 0 {
		doc["buffers"] = []any{map[string]any{"byteLength": binLen}}
	} else {
		delete(doc, "buffers")
	}

	jsonBytes, err := json.Marshal(doc)
	if err != nil {
		return "", 0, "", fmt.Errorf("%w: no se pudo serializar gltf", domain.ErrModelFormat)
	}
	jsonBytes = padBytes4(jsonBytes, 0x20)

	tmp, err := os.CreateTemp(outDir, "modelo-*.tmp")
	if err != nil {
		return "", 0, "", err
	}
	tmpName := tmp.Name()
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.Remove(tmpName)
		}
	}()

	n, sum, err := writeGLB(tmp, binTmpName, jsonBytes, binLen)
	if err != nil {
		_ = tmp.Close()
		return "", 0, "", err
	}
	if err := tmp.Close(); err != nil {
		return "", 0, "", err
	}
	if n > maxBytes {
		return "", 0, "", domain.ErrModelTooLarge
	}
	cleanupTmp = false
	return tmpName, n, sum, nil
}

func readFileHeader(fh *multipart.FileHeader, limit int64) ([]byte, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, limit))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) >= limit {
		return nil, domain.ErrModelTooLarge
	}
	return data, nil
}

func decodeGLTFJSON(data []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("%w: JSON gltf invalido", domain.ErrModelFormat)
	}
	if asset, ok := objectAt(doc, "asset"); !ok || stringAt(asset, "version") == "" {
		return nil, fmt.Errorf("%w: asset.version requerido", domain.ErrModelFormat)
	}
	return doc, nil
}

func buildAssetMap(files []*multipart.FileHeader, paths []string) (map[string]*multipart.FileHeader, error) {
	out := make(map[string]*multipart.FileHeader, len(files))
	for i, fh := range files {
		if fh == nil {
			continue
		}
		name := fh.Filename
		if i < len(paths) && strings.TrimSpace(paths[i]) != "" {
			name = paths[i]
		}
		p, err := cleanUploadPath(name)
		if err != nil {
			return nil, err
		}
		out[p] = fh
	}
	return out, nil
}

func cleanUploadPath(name string) (string, error) {
	cleaned := strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	cleaned = strings.TrimLeft(cleaned, "/")
	cleaned = path.Clean(cleaned)
	if cleaned == "." || cleaned == "" || path.IsAbs(cleaned) ||
		cleaned == ".." || strings.HasPrefix(cleaned, "../") ||
		strings.Contains(cleaned, ":") {
		return "", fmt.Errorf("%w: ruta de archivo invalida", domain.ErrModelFormat)
	}
	return cleaned, nil
}

func cleanGLTFURI(uri, baseDir string) (string, error) {
	raw := strings.TrimSpace(uri)
	if raw == "" {
		return "", fmt.Errorf("%w: uri vacia", domain.ErrModelFormat)
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%w: uri invalida %q", domain.ErrModelFormat, uri)
	}
	if parsed.Scheme != "" || parsed.Host != "" {
		return "", fmt.Errorf("%w: uri externa no soportada %q", domain.ErrModelFormat, uri)
	}
	decoded, err := url.PathUnescape(parsed.Path)
	if err != nil {
		return "", fmt.Errorf("%w: uri invalida %q", domain.ErrModelFormat, uri)
	}
	cleaned := path.Clean(path.Join(baseDir, strings.ReplaceAll(decoded, "\\", "/")))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) || strings.Contains(cleaned, ":") {
		return "", fmt.Errorf("%w: uri fuera de la carpeta %q", domain.ErrModelFormat, uri)
	}
	return cleaned, nil
}

func packBuffers(doc map[string]any, baseDir string, assets map[string]*multipart.FileHeader, dst *os.File, maxBytes int64) (map[int]int64, int64, error) {
	buffers, ok := arrayAt(doc, "buffers")
	if !ok || len(buffers) == 0 {
		return map[int]int64{}, 0, nil
	}

	offsets := make(map[int]int64, len(buffers))
	var binLen int64
	for i, raw := range buffers {
		buf, ok := raw.(map[string]any)
		if !ok {
			return nil, 0, fmt.Errorf("%w: buffer invalido", domain.ErrModelFormat)
		}
		uri := stringAt(buf, "uri")
		if uri == "" {
			return nil, 0, fmt.Errorf("%w: buffer externo requerido en .gltf", domain.ErrModelFormat)
		}
		var err error
		binLen, err = padFile4(dst, binLen, 0x00)
		if err != nil {
			return nil, 0, err
		}
		offsets[i] = binLen
		written, _, err := appendURIResource(uri, baseDir, assets, dst)
		if err != nil {
			return nil, 0, err
		}
		binLen += written
		if binLen > maxBytes {
			return nil, 0, domain.ErrModelTooLarge
		}
	}
	return offsets, binLen, nil
}

func rewriteBufferViews(doc map[string]any, bufferOffsets map[int]int64) error {
	views, ok := arrayAt(doc, "bufferViews")
	if !ok {
		return nil
	}
	for _, raw := range views {
		view, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: bufferView invalido", domain.ErrModelFormat)
		}
		bufferIndex := intValue(view["buffer"], 0)
		base, ok := bufferOffsets[bufferIndex]
		if !ok {
			return fmt.Errorf("%w: bufferView referencia buffer inexistente", domain.ErrModelFormat)
		}
		view["buffer"] = 0
		view["byteOffset"] = base + int64Value(view["byteOffset"], 0)
	}
	doc["bufferViews"] = views
	return nil
}

func packImages(doc map[string]any, baseDir string, assets map[string]*multipart.FileHeader, dst *os.File, binLen int64, maxBytes int64) (int64, error) {
	images, ok := arrayAt(doc, "images")
	if !ok {
		return binLen, nil
	}
	views, _ := arrayAt(doc, "bufferViews")

	for _, raw := range images {
		img, ok := raw.(map[string]any)
		if !ok {
			return 0, fmt.Errorf("%w: image invalida", domain.ErrModelFormat)
		}
		uri := stringAt(img, "uri")
		if uri == "" {
			continue
		}
		var err error
		binLen, err = padFile4(dst, binLen, 0x00)
		if err != nil {
			return 0, err
		}
		offset := binLen
		written, mimeType, err := appendURIResource(uri, baseDir, assets, dst)
		if err != nil {
			return 0, err
		}
		binLen += written
		if binLen > maxBytes {
			return 0, domain.ErrModelTooLarge
		}
		views = append(views, map[string]any{
			"buffer":     0,
			"byteOffset": offset,
			"byteLength": written,
		})
		img["bufferView"] = len(views) - 1
		if stringAt(img, "mimeType") == "" {
			img["mimeType"] = mimeType
		}
		delete(img, "uri")
	}

	doc["bufferViews"] = views
	doc["images"] = images
	return binLen, nil
}

func appendURIResource(uri, baseDir string, assets map[string]*multipart.FileHeader, dst *os.File) (int64, string, error) {
	if strings.HasPrefix(strings.ToLower(uri), "data:") {
		data, mimeType, err := decodeDataURI(uri)
		if err != nil {
			return 0, "", err
		}
		n, err := dst.Write(data)
		return int64(n), mimeType, err
	}

	rel, err := cleanGLTFURI(uri, baseDir)
	if err != nil {
		return 0, "", err
	}
	fh, ok := assets[rel]
	if !ok {
		return 0, "", fmt.Errorf("%w: selecciona la carpeta completa del .gltf; falta %s", domain.ErrModelFormat, rel)
	}
	src, err := fh.Open()
	if err != nil {
		return 0, "", err
	}
	defer src.Close()
	n, err := io.Copy(dst, src)
	if err != nil {
		return 0, "", err
	}
	return n, inferMime(rel), nil
}

func decodeDataURI(raw string) ([]byte, string, error) {
	comma := strings.IndexByte(raw, ',')
	if comma < 0 {
		return nil, "", fmt.Errorf("%w: data uri invalida", domain.ErrModelFormat)
	}
	meta := raw[5:comma]
	payload := raw[comma+1:]
	mimeType := "application/octet-stream"
	if parts := strings.Split(meta, ";"); len(parts) > 0 && parts[0] != "" {
		mimeType = parts[0]
	}
	if strings.Contains(strings.ToLower(meta), ";base64") {
		data, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, "", fmt.Errorf("%w: data uri base64 invalida", domain.ErrModelFormat)
		}
		return data, mimeType, nil
	}
	data, err := url.QueryUnescape(payload)
	if err != nil {
		return nil, "", fmt.Errorf("%w: data uri invalida", domain.ErrModelFormat)
	}
	return []byte(data), mimeType, nil
}

func inferMime(name string) string {
	if mt := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); mt != "" {
		return strings.Split(mt, ";")[0]
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".bin":
		return "application/octet-stream"
	case ".ktx2":
		return "image/ktx2"
	case ".basis":
		return "image/ktx2"
	default:
		return "application/octet-stream"
	}
}

func writeGLB(out *os.File, binPath string, jsonBytes []byte, binLen int64) (int64, string, error) {
	hasher := sha256.New()
	w := io.MultiWriter(out, hasher)

	totalLen := uint32(12 + 8 + len(jsonBytes))
	if binLen > 0 {
		totalLen += uint32(8 + binLen)
	}

	if err := binary.Write(w, binary.LittleEndian, glbMagic); err != nil {
		return 0, "", err
	}
	if err := binary.Write(w, binary.LittleEndian, glbVersion); err != nil {
		return 0, "", err
	}
	if err := binary.Write(w, binary.LittleEndian, totalLen); err != nil {
		return 0, "", err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(len(jsonBytes))); err != nil {
		return 0, "", err
	}
	if err := binary.Write(w, binary.LittleEndian, jsonChunk); err != nil {
		return 0, "", err
	}
	if _, err := w.Write(jsonBytes); err != nil {
		return 0, "", err
	}

	if binLen > 0 {
		if err := binary.Write(w, binary.LittleEndian, uint32(binLen)); err != nil {
			return 0, "", err
		}
		if err := binary.Write(w, binary.LittleEndian, binChunk); err != nil {
			return 0, "", err
		}
		binFile, err := os.Open(binPath)
		if err != nil {
			return 0, "", err
		}
		defer binFile.Close()
		if _, err := io.Copy(w, binFile); err != nil {
			return 0, "", err
		}
	}

	return int64(totalLen), hex.EncodeToString(hasher.Sum(nil)), nil
}

func padFile4(f *os.File, length int64, pad byte) (int64, error) {
	needed := (4 - (length % 4)) % 4
	if needed == 0 {
		return length, nil
	}
	padding := bytes.Repeat([]byte{pad}, int(needed))
	if _, err := f.Write(padding); err != nil {
		return 0, err
	}
	return length + needed, nil
}

func padBytes4(in []byte, pad byte) []byte {
	needed := (4 - (len(in) % 4)) % 4
	if needed == 0 {
		return in
	}
	return append(in, bytes.Repeat([]byte{pad}, needed)...)
}

func arrayAt(doc map[string]any, key string) ([]any, bool) {
	v, ok := doc[key]
	if !ok || v == nil {
		return nil, false
	}
	a, ok := v.([]any)
	return a, ok
}

func objectAt(doc map[string]any, key string) (map[string]any, bool) {
	v, ok := doc[key]
	if !ok || v == nil {
		return nil, false
	}
	o, ok := v.(map[string]any)
	return o, ok
}

func stringAt(doc map[string]any, key string) string {
	if v, ok := doc[key].(string); ok {
		return v
	}
	return ""
}

func intValue(v any, def int) int {
	switch n := v.(type) {
	case json.Number:
		if i, err := strconv.Atoi(n.String()); err == nil {
			return i
		}
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return def
}

func int64Value(v any, def int64) int64 {
	switch n := v.(type) {
	case json.Number:
		if i, err := strconv.ParseInt(n.String(), 10, 64); err == nil {
			return i
		}
	case float64:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	}
	return def
}
