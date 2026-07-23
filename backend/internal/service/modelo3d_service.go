package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"gitlab.com/syncbox/backend/internal/domain"
	"gitlab.com/syncbox/backend/internal/repository"
)

// Modelo3DService gestiona la subida y consulta de modelos 3D reusables.
type Modelo3DService struct {
	repo          *repository.Modelo3DRepo
	resourceRoot  string
	modelMaxBytes int64
}

// NewModelo3DService construye el servicio.
func NewModelo3DService(r *repository.Modelo3DRepo, resourceRoot string, modelMaxMB int) *Modelo3DService {
	if modelMaxMB <= 0 {
		modelMaxMB = 500
	}
	return &Modelo3DService{
		repo:          r,
		resourceRoot:  filepath.Clean(resourceRoot),
		modelMaxBytes: int64(modelMaxMB) * 1024 * 1024,
	}
}

// List devuelve los modelos disponibles.
func (s *Modelo3DService) List(ctx context.Context, search string) ([]domain.Modelo3D, error) {
	return s.repo.List(ctx, search, 200)
}

// Get devuelve un modelo por id.
func (s *Modelo3DService) Get(ctx context.Context, id uuid.UUID) (*domain.Modelo3D, error) {
	return s.repo.FindByID(ctx, id)
}

// Upload guarda un modelo .glb reusable. Tambien acepta .gltf con assets
// externos y lo empaqueta a .glb antes de deduplicar por sha256.
func (s *Modelo3DService) Upload(ctx context.Context, actor uuid.UUID, nombre, descripcion string, fh *multipart.FileHeader, filePath string, assets []*multipart.FileHeader, assetPaths []string) (*domain.Modelo3D, error) {
	if fh == nil {
		return nil, domain.ErrInvalidInput
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if ext != ".glb" && ext != ".gltf" {
		return nil, domain.ErrModelFormat
	}
	if fh.Size <= 0 {
		return nil, domain.ErrModelEmpty
	}
	if ext == ".glb" && fh.Size > s.modelMaxBytes {
		return nil, domain.ErrModelTooLarge
	}

	dir := filepath.Join(s.resourceRoot, "modelos3d")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}

	tmpName, n, sum, err := s.prepareModelUpload(fh, filePath, assets, assetPaths, dir)
	if err != nil {
		return nil, err
	}
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.Remove(tmpName)
		}
	}()

	// Dedup: si ya existe, devolverlo y descartar el archivo nuevo.
	existing, err := s.repo.FindBySHA256(ctx, sum)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	// Mover el tmp a su ubicación final por hash.
	finalRel := filepath.ToSlash(filepath.Join("modelos3d", sum+".glb"))
	finalAbs := filepath.Join(s.resourceRoot, filepath.FromSlash(finalRel))
	if err := os.Rename(tmpName, finalAbs); err != nil {
		return nil, err
	}
	cleanupTmp = false

	if strings.TrimSpace(nombre) == "" {
		nombre = strings.TrimSuffix(filepath.Base(fh.Filename), filepath.Ext(fh.Filename))
	}

	m := &domain.Modelo3D{
		Nombre: nombre, Descripcion: descripcion,
		Mime: "model/gltf-binary", TamanoBytes: n, SHA256: sum,
		StorageURI: finalRel,
	}
	if err := s.repo.Create(ctx, m, actor); err != nil {
		_ = os.Remove(finalAbs)
		return nil, err
	}
	return m, nil
}

func (s *Modelo3DService) prepareModelUpload(fh *multipart.FileHeader, filePath string, assets []*multipart.FileHeader, assetPaths []string, dir string) (string, int64, string, error) {
	switch strings.ToLower(filepath.Ext(fh.Filename)) {
	case ".glb":
		return s.copyGLBToTemp(fh, dir)
	case ".gltf":
		return convertUploadedGLTFToGLBTemp(fh, filePath, assets, assetPaths, dir, s.modelMaxBytes)
	default:
		return "", 0, "", domain.ErrModelFormat
	}
}

func (s *Modelo3DService) copyGLBToTemp(fh *multipart.FileHeader, dir string) (string, int64, string, error) {
	src, err := fh.Open()
	if err != nil {
		return "", 0, "", err
	}
	defer src.Close()

	tmp, err := os.CreateTemp(dir, "modelo-*.tmp")
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

	hasher := sha256.New()
	mw := io.MultiWriter(tmp, hasher)
	n, copyErr := io.Copy(mw, io.LimitReader(src, s.modelMaxBytes+1))
	closeErr := tmp.Close()
	if copyErr != nil {
		return "", 0, "", copyErr
	}
	if closeErr != nil {
		return "", 0, "", closeErr
	}
	if n > s.modelMaxBytes {
		return "", 0, "", domain.ErrModelTooLarge
	}
	cleanupTmp = false
	return tmpName, n, hex.EncodeToString(hasher.Sum(nil)), nil
}

// FilePath devuelve la ruta absoluta del archivo, validando que esté dentro
// del root de recursos (evita path traversal).
func (s *Modelo3DService) FilePath(m *domain.Modelo3D) (string, error) {
	cleanRel := filepath.Clean(filepath.FromSlash(m.StorageURI))
	if cleanRel == "." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) || filepath.IsAbs(cleanRel) {
		return "", domain.ErrInvalidInput
	}
	root, err := filepath.Abs(s.resourceRoot)
	if err != nil {
		return "", err
	}
	out, err := filepath.Abs(filepath.Join(root, cleanRel))
	if err != nil {
		return "", err
	}
	if out != root && !strings.HasPrefix(out, root+string(filepath.Separator)) {
		return "", domain.ErrInvalidInput
	}
	return out, nil
}

// Update permite renombrar/redescribir.
func (s *Modelo3DService) Update(ctx context.Context, actor, id uuid.UUID, nombre, descripcion *string) (*domain.Modelo3D, error) {
	if err := s.repo.Update(ctx, id, nombre, descripcion, actor); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// Delete borra un modelo si no está referenciado por equipos vivos.
func (s *Modelo3DService) Delete(ctx context.Context, id uuid.UUID) error {
	m, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	if path, err := s.FilePath(m); err == nil {
		_ = os.Remove(path)
	}
	return nil
}
