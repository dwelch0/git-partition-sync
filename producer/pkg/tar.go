package pkg

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (u *Uploader) tarRepos(toUpdate []*GitSync) error {
	const TAR_DIRECTORY = "tars"

	err := u.clean(TAR_DIRECTORY)
	if err != nil {
		return err
	}

	for _, gs := range toUpdate {
		// ensure the repo actually exists before trying to tar it
		if _, err := os.Stat(gs.repoPath); err != nil {
			return fmt.Errorf("Unable to tar files - %v", err.Error())
		}

		tarPath := fmt.Sprintf("%s/%s/%s.tar", u.workdir, TAR_DIRECTORY, gs.Source.ProjectName)
		f, err := os.Create(tarPath)
		if err != nil {
			return err
		}
		defer f.Close()

		tw := tar.NewWriter(f)
		defer tw.Close()

		// credit: https://medium.com/@skdomino/taring-untaring-files-in-go-6b07cf56bc07
		err = filepath.Walk(gs.repoPath, func(file string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !fi.Mode().IsRegular() {
				return nil
			}

			// create a new dir/file header
			header, err := tar.FileInfoHeader(fi, fi.Name())
			if err != nil {
				return err
			}

			// update the name to correctly reflect the desired destination when untaring
			header.Name = strings.TrimPrefix(strings.Replace(file, gs.repoPath, "", -1), string(filepath.Separator))

			// write the header
			if err := tw.WriteHeader(header); err != nil {
				return err
			}

			// open files for taring
			f, err := os.Open(file)
			if err != nil {
				return err
			}

			// copy file data into tar writer
			if _, err := io.Copy(tw, f); err != nil {
				return err
			}

			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			f.Close()

			return nil
		})

		if err != nil {
			return err
		}

		gs.tarPath = tarPath
	}

	return nil
}
