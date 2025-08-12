package job

import (
	"context"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/spf13/afero"
	"github.com/spf13/viper"

	"github.com/wei840222/simple-file-server/config"
)

func TestFileActivity_ListByPattern(t *testing.T) {
	memFs := afero.NewMemMapFs()
	root := "/testroot"
	act := &FileActivities{fs: memFs}

	_ = memFs.MkdirAll(root+"/dir1", 0755)
	_ = memFs.MkdirAll(root+"/dir2", 0755)
	_ = afero.WriteFile(memFs, root+"/file.txt", []byte("hello"), 0644)
	_ = afero.WriteFile(memFs, root+"/dir1/file1.txt", []byte("hello"), 0644)
	_ = afero.WriteFile(memFs, root+"/dir1/._file1.txt", []byte("world"), 0644)
	_ = afero.WriteFile(memFs, root+"/dir1/.DS_Store", []byte("log"), 0644)

	Convey("When ListByPattern is called", t, func() {
		viper.Set(config.KeyFileRoot, root)
		files, err := act.ListByPattern(context.Background(), []string{`^\._.+`, `^\.DS_Store$`})

		So(err, ShouldBeNil)
		So(files, ShouldContain, root+"/dir1/._file1.txt")
		So(files, ShouldContain, root+"/dir1/.DS_Store")
		So(len(files), ShouldEqual, 2)
	})

	Convey("When the root directory does not exist", t, func() {
		viper.Set(config.KeyFileRoot, "/notexist")
		files, err := act.ListByPattern(context.Background(), []string{})

		So(err, ShouldNotBeNil)
		So(files, ShouldBeEmpty)
	})
}

func TestFileActivity_Delete(t *testing.T) {
	memFs := afero.NewMemMapFs()
	root := "/testroot"
	act := &FileActivities{fs: memFs}

	_ = memFs.MkdirAll(root+"/dir1", 0755)
	_ = afero.WriteFile(memFs, root+"/dir1/._file1.txt", []byte("hello"), 0644)

	Convey("When Delete is called", t, func() {
		viper.Set(config.KeyFileRoot, root)
		err := act.Delete(context.Background(), root+"/dir1/._file1.txt")

		So(err, ShouldBeNil)

		files, err := act.ListByPattern(context.Background(), []string{`^\._.+`})
		So(err, ShouldBeNil)
		So(files, ShouldBeEmpty)
	})
}
