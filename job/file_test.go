package job

import (
	"context"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/spf13/afero"
)

func TestFileActivity_ListByPattern(t *testing.T) {
	memFs := afero.NewMemMapFs()
	act := &FileActivities{fs: memFs}

	_ = memFs.MkdirAll("dir1", 0755)
	_ = memFs.MkdirAll("dir2", 0755)
	_ = afero.WriteFile(memFs, "file.txt", []byte("hello"), 0644)
	_ = afero.WriteFile(memFs, "dir1/file1.txt", []byte("hello"), 0644)
	_ = afero.WriteFile(memFs, "dir1/._file1.txt", []byte("world"), 0644)
	_ = afero.WriteFile(memFs, "dir1/.DS_Store", []byte("log"), 0644)

	Convey("When ListByPattern is called", t, func() {
		files, err := act.ListByPattern(context.Background(), []string{`^\._.+`, `^\.DS_Store$`})

		So(err, ShouldBeNil)
		So(files, ShouldContain, "dir1/._file1.txt")
		So(files, ShouldContain, "dir1/.DS_Store")
		So(len(files), ShouldEqual, 2)
	})
}

func TestFileActivity_Delete(t *testing.T) {
	memFs := afero.NewMemMapFs()
	act := &FileActivities{fs: memFs}

	_ = memFs.MkdirAll("dir1", 0755)
	_ = afero.WriteFile(memFs, "dir1/._file1.txt", []byte("hello"), 0644)

	Convey("When Delete is called", t, func() {
		err := act.Delete(context.Background(), "dir1/._file1.txt")

		So(err, ShouldBeNil)

		files, err := act.ListByPattern(context.Background(), []string{`^\._.+`})
		So(err, ShouldBeNil)
		So(files, ShouldBeEmpty)
	})
}
