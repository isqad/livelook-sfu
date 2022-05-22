package core

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxMemoryForParse = 32 << 20

var (
	errIDMustBeGreaterThanZero = errors.New("ID must be greater or equal to zero")
)

type ImageDBStorer interface {
	Save(img *Image) error
}

type ImageUploadHandler interface {
	HandleUpload(*http.Request, ImageDBStorer) error
	URL() (string, error)
}

type ImageUploadOptions struct {
	ImageDir         string // Directory on disk where the picture is saved
	ImagePrefixPath  string // Prefix of image path
	ImageURLPattern  string // URL pattern e.g. /rs/fit/760/760/sm/0/plain/local://%s?%d
	ImageFormKey     string
	ImageDescFormKey string
	ImageRootDir     string // Usually is a local directory for saved images
}

type ImageOptions struct {
	ImageUploadOptions

	ID          int64
	Filename    string
	Description string
	UpdatedAt   time.Time
}

type Image struct {
	ImageOptions

	idPartitions []string
}

func NewImage(options ImageOptions) *Image {
	img := &Image{
		ImageOptions: options,
	}

	return img
}

func (img *Image) HandleUpload(r *http.Request, imgDB ImageDBStorer) error {
	err := r.ParseMultipartForm(maxMemoryForParse)
	if err != nil {
		return err
	}
	uploadedFile, fileHeader, err := r.FormFile(img.ImageFormKey)
	if err != nil {
		return err
	}

	defer uploadedFile.Close()

	img.Filename = fileHeader.Filename
	img.Description = r.FormValue(img.ImageDescFormKey)

	if err := imgDB.Save(img); err != nil {
		return err
	}

	idParts, err := img.idParts()
	if err != nil {
		return err
	}
	img.idPartitions = idParts

	saveDir := img.ImageRootDir + img.ImageDir
	imgDir, err := img.dir(saveDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		return err
	}
	photoPath, _ := img.path(saveDir)
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

func (img *Image) URL() (string, error) {
	imgPath, err := img.path(img.ImagePrefixPath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(img.ImageURLPattern, imgPath, img.UpdatedAt.Unix()), nil
}

func (img *Image) path(prefixDir string) (string, error) {
	photoDir, err := img.dir(prefixDir)
	if err != nil {
		return "", err
	}
	dirs, err := img.idParts()
	if err != nil {
		return "", err
	}
	fileName := dirs[len(dirs)-1] + "_original" + filepath.Ext(img.Filename)

	return photoDir + "/" + fileName, nil
}

func (img *Image) dir(prefixDir string) (string, error) {
	dirs, err := img.idParts()
	if err != nil {
		return "", err
	}
	return prefixDir + "/" + strings.Join(dirs[:(len(dirs)-1)], `/`), nil
}

func (img *Image) idParts() ([]string, error) {
	if len(img.idPartitions) > 0 {
		return img.idPartitions, nil
	}

	return idPartition(img.ID)
}

func idPartition(id int64) ([]string, error) {
	if id <= 0 {
		return nil, errIDMustBeGreaterThanZero
	}

	var parts []string

	idStr := fmt.Sprintf("%09d", id)

	acc := ""
	idStrLen := len(idStr)
	tailLen := idStrLen
	nextPos := 0

	for pos, c := range idStr {
		acc += string(c)
		nextPos = pos + 1

		if nextPos%imgPartLen == 0 {
			parts = append(parts, acc)
			acc = ""
			tailLen -= imgPartLen
		} else if tailLen > 0 && tailLen < imgPartLen {
			parts = append(parts, idStr[pos:idStrLen])
			break
		}
	}

	return parts, nil
}
