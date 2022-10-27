package pkg

import (
	"fmt"
	"log"
	"os"

	"filippo.io/age"
)

// utilizes x25519 to output encrypted tars
func (u *Uploader) encryptRepoTars(toUpdate []*GitSync) error {
	const ENCRYPT_DIRECTORY = "encrypted"

	err := u.clean(ENCRYPT_DIRECTORY)
	if err != nil {
		return err
	}

	recipient, err := age.ParseX25519Recipient(u.publicKey)
	if err != nil {
		log.Fatalf("Failed to parse public key %q: %v", u.publicKey, err)
	}

	for _, gs := range toUpdate {
		encryptPath := fmt.Sprintf("%s/%s/%s.tar.age", u.workdir, ENCRYPT_DIRECTORY, gs.Source.ProjectName)
		f, err := os.Create(encryptPath)
		if err != nil {
			return err
		}
		defer f.Close()

		// read in tar data
		tarBytes, err := os.ReadFile(gs.tarPath)
		if err != nil {
			return err
		}

		// encrypt
		encWriter, err := age.Encrypt(f, recipient)
		if err != nil {
			return err
		}
		encWriter.Write(tarBytes)

		if err := encWriter.Close(); err != nil {
			return err
		}
		gs.encryptPath = encryptPath
	}

	return nil
}
