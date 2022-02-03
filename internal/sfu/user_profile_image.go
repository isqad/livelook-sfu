package sfu

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

const (
	usersProfileImageDir       = "/static/user_profile_images"
	userProfileImagePrefixPath = "/user_profile_images"
	userProgileImageURLPattern = "/rs/fit/760/760/sm/0/plain/local://%s?%d"
	imgPartLen                 = 3
	fileFormKey                = "image"
)

// UserProfileImageSaver сохраняет аватар в БД
type UserProfileImageSaver interface {
	// Save сохраняет информацию об аватаре пользователя в БД и возвращает ее ID или ошибку
	Save(img *UserProfileImage) (int64, error)
}

// UserProfileImage аватарки пользователя
type UserProfileImage struct {
	ID        int64     `json:"id,omitempty" db:"id"`
	Position  int       `json:"position,omitempty" db:"position"`
	Filename  string    `json:"filename" db:"filename"`
	UserID    string    `json:"user_id,omitempty" db:"user_id"`
	CreatedAt time.Time `json:"-" db:"created_at"`

	rootDir      string
	idPartitions []string
}

func NewUserProfileImage(userID string, options ...string) *UserProfileImage {
	img := &UserProfileImage{UserID: userID, CreatedAt: time.Now().UTC()}
	if len(options) > 0 && options[0] != "" {
		img.rootDir = options[0]
	}

	return img
}

// UploadHandle загружает картинку по http из клиентского запроса
// TODO: возможно, надо ограничить по размеру загружаемого файла
func (p *UserProfileImage) UploadHandle(r *http.Request, dbSaver UserProfileImageSaver) error {
	err := r.ParseMultipartForm(32 << 20) // maxMemory 32MB
	if err != nil {
		return err
	}
	uploadedFile, fileHeader, err := r.FormFile(fileFormKey)
	if err != nil {
		return err
	}

	defer uploadedFile.Close()

	p.Filename = fileHeader.Filename

	imgID, err := dbSaver.Save(p)
	if err != nil {
		return err
	}

	p.ID = imgID
	idParts, err := p.IDPartitions()
	if err != nil {
		return err
	}
	p.idPartitions = idParts

	saveDir := p.rootDir + usersProfileImageDir
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
func (p *UserProfileImage) URL() (string, error) {
	imgPath, err := p.Path(userProfileImagePrefixPath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(userProgileImageURLPattern, imgPath, p.CreatedAt.UnixNano()), nil
}

// Path возвращает полный путь до имени файла картинки
func (p *UserProfileImage) Path(prefixDir string) (string, error) {
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
func (p *UserProfileImage) Dir(prefixDir string) (string, error) {
	dirs, err := p.IDPartitions()
	if err != nil {
		return "", err
	}
	return prefixDir + "/" + strings.Join(dirs[:(len(dirs)-1)], `/`), nil
}

// IDPartitions считает из ID картинки партицию - путь куда будет положена картинка на диск.
// Партиции важны, так как позволяют решить проблему израсходывания inode при складывании файлов в одну директорию на диске
func (p *UserProfileImage) IDPartitions() ([]string, error) {
	if len(p.idPartitions) > 0 {
		return p.idPartitions, nil
	}

	return idPartition(p.ID)
}

func idPartition(id int64) ([]string, error) {
	if id <= 0 {
		return nil, errors.New("ID must be greater or equal to zero")
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
