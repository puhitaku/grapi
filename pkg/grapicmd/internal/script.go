package internal

import (
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/afero"

	"github.com/izumin5210/grapi/pkg/grapicmd/internal/module"
	"github.com/izumin5210/grapi/pkg/grapicmd/util/fs"
)

// Script represents an user-defined command.
type Script interface {
	Name() string
	Build() error
	Run() error
}

// ScriptFactory is a factory object for creating Script objects.
type ScriptFactory interface {
	Create(entryFilePath string) Script
}

// NewScriptFactory creates a new ScriptFactory instance.
func NewScriptFactory(fs afero.Fs, commandFactory module.CommandFactory, rootDir string) ScriptFactory {
	return &scriptFactory{
		fs:             fs,
		commandFactory: commandFactory,
		rootDir:        rootDir,
	}
}

type scriptFactory struct {
	fs             afero.Fs
	commandFactory module.CommandFactory
	rootDir        string
}

func (f *scriptFactory) Create(entryFilePath string) Script {
	return &script{
		fs:             f.fs,
		commandFactory: f.commandFactory,
		rootDir:        f.rootDir,
		entryFilePath:  entryFilePath,
	}
}

type script struct {
	fs                       afero.Fs
	commandFactory           module.CommandFactory
	rootDir                  string
	entryFilePath            string
	binName, binDir, binPath string
}

func (s *script) Name() string {
	return s.getBinName()
}

func (s *script) Build() error {
	err := fs.CreateDirIfNotExists(s.fs, s.getBinDir())
	if err != nil {
		return errors.WithStack(err)
	}

	cmd := s.commandFactory.Create([]string{"go", "build", "-v", "-o=" + s.getBinPath(), s.entryFilePath})
	_, err = cmd.ConnectIO().SetDir(s.rootDir).Exec()
	if err != nil {
		return errors.Wrapf(err, "failed to build %q", s.entryFilePath)
	}

	return nil
}

func (s *script) Run() error {
	cmd := s.commandFactory.Create([]string{s.getBinPath()})
	_, err := cmd.ConnectIO().SetDir(s.rootDir).Exec()
	return errors.WithStack(err)
}

func (s *script) getBinName() string {
	if s.binName == "" {
		s.binName = filepath.Base(filepath.Dir(s.entryFilePath))
	}
	return s.binName
}

func (s *script) getBinDir() string {
	if s.binDir == "" {
		s.binDir = filepath.Join(s.rootDir, "bin")
	}
	return s.binDir
}

func (s *script) getBinPath() string {
	if s.binPath == "" {
		s.binPath = filepath.Join(s.getBinDir(), s.getBinName())
	}
	return s.binPath
}
