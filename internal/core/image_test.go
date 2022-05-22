package core

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIDPartition(t *testing.T) {
	got, err := idPartition(1)
	assert.Nil(t, err)

	if strings.Join(got, `/`) != "000/000/001" {
		t.Errorf("IdPartition(1) = %s; want 000/000/001", got)
	}

	_, err = idPartition(-1)
	assert.Equal(t, errIDMustBeGreaterThanZero, err)

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

func TestURL(t *testing.T) {
	u, _ := time.Parse("Jan 2, 2006 at 3:04pm (MST)", "Jan 2, 2006 at 3:04pm (MST)")
	expectedUnixTimestamp := u.Unix()
	pattern := "/rs/fit/760/760/sm/0/plain/local://%s?%d"

	opts := ImageOptions{
		ImageUploadOptions: ImageUploadOptions{
			ImageDir:        "/foo/bar",
			ImagePrefixPath: "/bar",
			ImageURLPattern: pattern,
		},
		ID:        100_500,
		Filename:  "foo_bar.jpg",
		UpdatedAt: u,
	}
	img := NewImage(opts)
	url, err := img.URL()
	assert.Nil(t, err)

	assert.Equal(
		t,
		fmt.Sprintf(pattern, "/bar/000/100/500_original.jpg", expectedUnixTimestamp),
		url,
	)
}
