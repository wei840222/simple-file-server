package server

import (
	"context"
	"os"

	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"golang.org/x/net/webdav"

	"github.com/wei840222/simple-file-server/config"
)

func NewAferoFS() (afero.Fs, error) {
	fs := afero.NewOsFs()

	exist, err := afero.DirExists(fs, viper.GetString(config.KeyFileRoot))
	if err != nil {
		return nil, err
	}

	if !exist {
		fs.MkdirAll(viper.GetString(config.KeyFileRoot), os.ModePerm)
	}

	return afero.NewBasePathFs(fs, viper.GetString(config.KeyFileRoot)), nil
}

func AferoFSWebdavAdapter(fs afero.Fs) webdav.FileSystem {
	return &aferoFSWebdavAdapter{fs: fs}
}

type aferoFSWebdavAdapter struct {
	fs afero.Fs
}

func (a *aferoFSWebdavAdapter) Mkdir(_ context.Context, name string, perm os.FileMode) error {
	return a.fs.Mkdir(name, perm)
}

func (a *aferoFSWebdavAdapter) OpenFile(_ context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	return a.fs.OpenFile(name, flag, perm)
}

func (a *aferoFSWebdavAdapter) RemoveAll(_ context.Context, name string) error {
	return a.fs.RemoveAll(name)
}

func (a *aferoFSWebdavAdapter) Rename(_ context.Context, oldName, newName string) error {
	return a.fs.Rename(oldName, newName)
}

func (a *aferoFSWebdavAdapter) Stat(_ context.Context, name string) (os.FileInfo, error) {
	return a.fs.Stat(name)
}
