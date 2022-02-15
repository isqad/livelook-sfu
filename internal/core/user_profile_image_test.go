package core

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIDPartition(t *testing.T) {
	got, err := idPartition(1)
	assert.Nil(t, err)

	if strings.Join(got, `/`) != "000/000/001" {
		t.Errorf("IdPartition(1) = %s; want 000/000/001", got)
	}

	_, err = idPartition(-1)
	assert.NotNil(t, err)

	got, err = idPartition(int64(1234567891234568))
	assert.Nil(t, err)
	if strings.Join(got, `/`) != "123/456/789/123/456/8" {
		t.Errorf("IdPartition(1234567891234568) = %s; want 123/456/789/123/456/8", got)
	}

	got, err = idPartition(int64(12345678912))
	assert.Nil(t, err)
	if strings.Join(got, `/`) != "123/456/789/12" {
		t.Errorf("IdPartition(12345678912) = %s; want 123/456/789/12", got)
	}
}

func TestPhotoUrl(t *testing.T) {
	photo := &UserProfileImage{
		ID:       100500,
		Filename: "foo_bar.jpg",
	}

	photoPath, err := photo.Path(userProfileImagePrefixPath)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	expectedPhotoUrl := fmt.Sprintf(
		"/rs/fit/760/760/sm/0/plain/local://%s?%d",
		photoPath,
		photo.CreatedAt.UnixNano(),
	)

	url, err := photo.URL()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if url != expectedPhotoUrl {
		t.Errorf("Expected %v, got: %v", expectedPhotoUrl, url)
	}
}

type mockUserProfileImageSaver struct {
	DesiredID    int64
	DesiredError error
}

func (m mockUserProfileImageSaver) Save(img *UserProfileImage) (int64, error) {
	img.ID = m.DesiredID
	return m.DesiredID, m.DesiredError
}

func TestUploadHandle(t *testing.T) {
	imgContent, err := ioutil.ReadFile("../../fixtures/pixel.jpg")
	assert.Nil(t, err)

	bodyReader := strings.NewReader(
		"--foo\r\n" +
			"Content-Disposition: form-data; name=\"" + fileFormKey + "\"; filename=\"pixel.jpg\"\r\n" +
			"Content-Type: image/jpeg\r\n" +
			"\r\n" + string(imgContent) +
			"\r\n--foo--\r\n",
	)

	request, _ := http.NewRequest(http.MethodPost, "/api/v1/profile/images", bodyReader)
	request.Header.Add("Content-Type", "multipart/form-data; boundary=foo")
	response := httptest.NewRecorder()

	tmpDir, err := os.MkdirTemp("", "test_upload_handle")
	assert.Nil(t, err)
	defer os.RemoveAll(tmpDir)

	img := NewUserProfileImage("some-user-id", tmpDir)
	dbSaver := mockUserProfileImageSaver{4242420, nil}
	err = img.UploadHandle(request, dbSaver)
	assert.Nil(t, err)

	status := response.Code
	assert.Equal(t, http.StatusOK, status)

	assert.Equal(t, int64(4242420), img.ID)
	assert.Equal(t, "pixel.jpg", img.Filename)

	// Убедимся, что файл действительно загрузился по нужному пути
	_, err = os.Stat(tmpDir + usersProfileImageDir + "/004/242/420_original.jpg")
	assert.Nil(t, err)
}
