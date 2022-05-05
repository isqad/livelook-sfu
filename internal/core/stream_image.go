package core

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	streamImageDir           = "/static/stream_images"
	streamImagePrefixPath    = "/stream_images"
	streamImageURLPattern    = "/rs/fit/760/760/sm/0/plain/local://%s?%d"
	streamImageFormKey       = "stream_image"
	streamDescriptionFormKey = "stream_title"
)

// StreamImageSaver сохраняет картинку в БД
type StreamImageSaver interface {
	Save(img *StreamImage) error
}

// StreamImage is cover of stream
type StreamImage struct {
	Session     *Session
	Description string
	Filename    string

	rootDir      string
	idPartitions []string
}

func NewStreamImage(session *Session, options ...string) *StreamImage {
	img := &StreamImage{Session: session}
	if len(options) > 0 && options[0] != "" {
		img.rootDir = options[0]
	}

	if session.ImageFilename != nil {
		img.Filename = *session.ImageFilename
	}

	return img
}

// UploadHandle загружает картинку по http из клиентского запроса
// TODO: возможно, надо ограничить по размеру загружаемого файла
func (p *StreamImage) UploadHandle(r *http.Request, dbSaver StreamImageSaver) error {
	err := r.ParseMultipartForm(32 << 20) // maxMemory 32MB
	if err != nil {
		return err
	}
	uploadedFile, fileHeader, err := r.FormFile(streamImageFormKey)
	if err != nil {
		return err
	}

	defer uploadedFile.Close()

	p.Filename = fileHeader.Filename
	p.Description = r.FormValue(streamDescriptionFormKey)

	if err := dbSaver.Save(p); err != nil {
		return err
	}

	idParts, err := p.IDPartitions()
	if err != nil {
		return err
	}
	p.idPartitions = idParts

	saveDir := p.rootDir + streamImageDir
	imgDir, err := p.Dir(saveDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		return err
	}
	photoPath, _ := p.Path(saveDir)
	f, err := os.OpenFile(photoPath, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, uploadedFile); err != nil {
		return err
	}

	return nil
}

// URL возвращает ссылку на картинку
func (s *StreamImage) URL() (string, error) {
	imgPath, err := s.Path(streamImagePrefixPath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(streamImageURLPattern, imgPath, s.Session.UpdatedAt.Unix()), nil
}

// Path возвращает полный путь до имени файла картинки
func (p *StreamImage) Path(prefixDir string) (string, error) {
	photoDir, err := p.Dir(prefixDir)
	if err != nil {
		return "", err
	}
	dirs, err := p.IDPartitions()
	if err != nil {
		return "", err
	}
	fileName := dirs[len(dirs)-1] + "_original" + filepath.Ext(p.Filename)

	return photoDir + "/" + fileName, nil
}

// Dir формирует путь куда будет сохранен файл картинки
func (p *StreamImage) Dir(prefixDir string) (string, error) {
	dirs, err := p.IDPartitions()
	if err != nil {
		return "", err
	}
	return prefixDir + "/" + strings.Join(dirs[:(len(dirs)-1)], `/`), nil
}

// IDPartitions считает из ID картинки партицию - путь куда будет положена картинка на диск.
// Партиции важны, так как позволяют решить проблему израсходывания inode при складывании файлов в одну директорию на диске
func (p *StreamImage) IDPartitions() ([]string, error) {
	if len(p.idPartitions) > 0 {
		return p.idPartitions, nil
	}

	return idPartition(p.Session.ID)
}
